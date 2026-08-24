package pairing

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
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
	mu          sync.Mutex
	wsURL       string
	ttl         time.Duration
	clock       Clock
	random      io.Reader
	current     *credential
	invalidated bool
	leases      map[uint64]*leaseState
	nextLeaseID uint64
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

func (m *Manager) issueLocked(sessionID string) (Payload, error) {
	var raw [32]byte
	if _, err := io.ReadFull(m.random, raw[:]); err != nil {
		return Payload{}, fmt.Errorf("generate pair credential: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	digest := sha256.Sum256([]byte(token))

	expiresAt := m.clock().Add(m.ttl)
	m.current = &credential{sessionID: sessionID, digest: digest, expiresAt: expiresAt}
	m.invalidated = false

	return Payload{
		Version:   PayloadVersion,
		WSURL:     m.wsURL,
		SessionID: sessionID,
		PairToken: token,
		ExpiresAt: expiresAt,
	}, nil
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
