package pairing

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrNoCredential = errors.New("no active credential")
)

// Clock supplies the current time and can be replaced by deterministic tests.
type Clock func() time.Time

// Option configures a Manager before it is used.
type Option func(*Manager)

// WithClock replaces the wall clock.
func WithClock(clock Clock) Option {
	return func(manager *Manager) {
		if clock != nil {
			manager.clock = clock
		}
	}
}

// WithRandom replaces the credential random source.
func WithRandom(reader io.Reader) Option {
	return func(manager *Manager) {
		if reader != nil {
			manager.random = reader
		}
	}
}

// WithPersistenceFile enables restart recovery for the current Session.
// The file contains only this Session's pairing record and must be protected
// by the caller's local filesystem permissions.
func WithPersistenceFile(path string) Option {
	return func(manager *Manager) {
		manager.persistenceFile = strings.TrimSpace(path)
	}
}

type credential struct {
	sessionID string
	digest    [sha256.Size]byte
	expiresAt time.Time
}

type leaseState struct {
	done chan struct{}
	once sync.Once
}

// Manager owns the current short-lived pairing credential and authenticated leases.
type Manager struct {
	mu              sync.Mutex
	wsURL           string
	ttl             time.Duration
	clock           Clock
	random          io.Reader
	persistenceFile string
	current         *credential
	invalidated     bool
	leases          map[uint64]*leaseState
	nextLeaseID     uint64
}

// Lease represents one established authenticated connection.
type Lease struct {
	manager *Manager
	id      uint64
	state   *leaseState
	once    sync.Once
}

// NewManager validates stable pairing configuration.
func NewManager(wsURL string, ttl time.Duration, options ...Option) (*Manager, error) {
	parsed, err := url.Parse(wsURL)
	if err != nil || (parsed.Scheme != "ws" && parsed.Scheme != "wss") || parsed.Host == "" {
		return nil, errors.New("invalid advertised WebSocket URL")
	}
	if ttl <= 0 {
		return nil, errors.New("pair token TTL must be positive")
	}
	manager := &Manager{
		wsURL:  wsURL,
		ttl:    ttl,
		clock:  time.Now,
		random: rand.Reader,
		leases: make(map[uint64]*leaseState),
	}
	for _, option := range options {
		option(manager)
	}
	return manager, nil
}

// Issue replaces the credential accepted for new connections.
func (m *Manager) Issue(sessionID string) (Payload, error) {
	if strings.TrimSpace(sessionID) == "" {
		return Payload{}, errors.New("session ID is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	return m.issueLocked(sessionID)
}

// issueLocked reuses the persisted credential when it still covers sessionID so
// a restart keeps the previously scanned QR code valid. Persistence is only a
// convenience: every I/O failure degrades to "no recovery, no save" and never
// prevents the Session from starting.
func (m *Manager) issueLocked(sessionID string) (Payload, error) {
	if m.persistenceFile != "" {
		if payload, ok := m.loadPersistedLocked(sessionID); ok {
			m.current = credentialFromPayload(payload)
			m.invalidated = false
			return payload, nil
		}
	}
	var raw [32]byte
	if _, err := io.ReadFull(m.random, raw[:]); err != nil {
		return Payload{}, fmt.Errorf("generate pair credential: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])

	expiresAt := m.clock().Add(m.ttl)
	payload := Payload{
		Version:   PayloadVersion,
		SessionID: sessionID,
		PairToken: token,
		ExpiresAt: expiresAt,
	}
	if m.persistenceFile != "" {
		// A failed save only costs restart recovery; the credential below still
		// authorizes clients for the current run.
		_ = m.savePersistedLocked(payload)
	}
	m.current = credentialFromPayload(payload)
	m.invalidated = false

	payload.WSURL = m.wsURL
	return payload, nil
}

func credentialFromPayload(payload Payload) *credential {
	return &credential{
		sessionID: payload.SessionID,
		digest:    sha256.Sum256([]byte(payload.PairToken)),
		expiresAt: payload.ExpiresAt,
	}
}

type persistedPayload struct {
	SessionID string    `json:"sessionId"`
	PairToken string    `json:"pairToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// loadPersistedLocked reports whether the stored record can still authorize
// sessionID. An unreadable, damaged, mismatched or expired record simply yields
// false so the caller issues a fresh credential.
func (m *Manager) loadPersistedLocked(sessionID string) (Payload, bool) {
	data, err := os.ReadFile(m.persistenceFile)
	if err != nil {
		return Payload{}, false
	}
	var stored persistedPayload
	if err := json.Unmarshal(data, &stored); err != nil {
		return Payload{}, false
	}
	if stored.SessionID != sessionID || stored.PairToken == "" || !m.clock().Before(stored.ExpiresAt) {
		return Payload{}, false
	}
	// WSURL always comes from the current process so a restart on a new
	// listen/advertise address still hands out a reachable endpoint.
	return Payload{
		Version:   PayloadVersion,
		WSURL:     m.wsURL,
		SessionID: stored.SessionID,
		PairToken: stored.PairToken,
		ExpiresAt: stored.ExpiresAt,
	}, true
}

// PruneExpired removes pairing records under dir whose credential already
// expired. Records that are unreadable, unparsable or still valid are left
// untouched so a concurrent Session never loses its credential.
func PruneExpired(dir string, now time.Time) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read pairing directory: %w", err)
	}
	var failures []error
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var stored persistedPayload
		if err := json.Unmarshal(data, &stored); err != nil {
			continue
		}
		if now.Before(stored.ExpiresAt) {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (m *Manager) savePersistedLocked(payload Payload) error {
	if err := os.MkdirAll(filepath.Dir(m.persistenceFile), 0o700); err != nil {
		return fmt.Errorf("create pairing directory: %w", err)
	}
	data, err := json.Marshal(persistedPayload{
		SessionID: payload.SessionID,
		PairToken: payload.PairToken,
		ExpiresAt: payload.ExpiresAt,
	})
	if err != nil {
		return fmt.Errorf("encode pairing file: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(m.persistenceFile), ".pairing-*.tmp")
	if err != nil {
		return fmt.Errorf("create pairing file: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("protect pairing file: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write pairing file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close pairing file: %w", err)
	}
	if err := os.Rename(temporaryName, m.persistenceFile); err != nil {
		return fmt.Errorf("replace pairing file: %w", err)
	}
	return nil
}

// RotateIfExpired replaces an expired credential without disturbing established leases.
func (m *Manager) RotateIfExpired() (Payload, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == nil || m.invalidated {
		return Payload{}, false, ErrNoCredential
	}
	if m.clock().Before(m.current.expiresAt) {
		return Payload{}, false, nil
	}
	sessionID := m.current.sessionID
	payload, err := m.issueLocked(sessionID)
	if err != nil {
		return Payload{}, false, err
	}
	return payload, true, nil
}

// Invalidate prevents new authentication while preserving established leases.
func (m *Manager) Invalidate() {
	m.mu.Lock()
	m.invalidated = true
	m.current = nil
	m.mu.Unlock()
}

// Revoke prevents new authentication and signals every established lease.
func (m *Manager) Revoke() {
	m.mu.Lock()
	m.invalidated = true
	m.current = nil
	for id, state := range m.leases {
		state.once.Do(func() { close(state.done) })
		delete(m.leases, id)
	}
	m.mu.Unlock()
}

// Authenticate validates a credential and registers an established lease.
func (m *Manager) Authenticate(sessionID, token string) (*Lease, error) {
	digest := sha256.Sum256([]byte(token))

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == nil || m.invalidated || !m.clock().Before(m.current.expiresAt) || sessionID != m.current.sessionID || subtle.ConstantTimeCompare(digest[:], m.current.digest[:]) != 1 {
		return nil, ErrUnauthorized
	}

	m.nextLeaseID++
	state := &leaseState{done: make(chan struct{})}
	m.leases[m.nextLeaseID] = state
	return &Lease{manager: m, id: m.nextLeaseID, state: state}, nil
}

// Done is closed when the credential owner revokes this established lease.
func (l *Lease) Done() <-chan struct{} {
	return l.state.done
}

// Close releases the lease without affecting other authenticated connections.
func (l *Lease) Close() {
	if l == nil || l.manager == nil {
		return
	}
	l.once.Do(func() {
		l.manager.mu.Lock()
		delete(l.manager.leases, l.id)
		l.manager.mu.Unlock()
	})
}
