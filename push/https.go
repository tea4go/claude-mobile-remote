package push

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const EventTTL = 5 * time.Minute

var topicPattern = regexp.MustCompile(`^cmr_[0-9a-f]{32}$`)
var eventPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Event deliberately excludes Session IDs, pairing credentials and message text.
type Event struct {
	EventID   string    `json:"eventId"`
	Topic     string    `json:"topic"`
	Kind      string    `json:"kind"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func validKind(kind string) bool {
	return kind == "approval" || kind == "question" || kind == "completed" || kind == "failed" || kind == "stopped"
}
func (e Event) valid(now time.Time) bool {
	return eventPattern.MatchString(e.EventID) && topicPattern.MatchString(e.Topic) && validKind(e.Kind) && e.ExpiresAt.After(now) && !e.ExpiresAt.After(now.Add(EventTTL+15*time.Second))
}

// HTTPS sends notification-only events to edo. Redirects are never followed.
type HTTPS struct {
	endpoint, token string
	client          *http.Client
}

func NewHTTPS(endpoint, token string) (*HTTPS, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || len(token) < 32 || len(token) > 512 || strings.ContainsAny(token, " \r\n\t") {
		return nil, errors.New("notification service requires an HTTPS URL and a dedicated token of at least 32 characters")
	}
	return &HTTPS{endpoint: endpoint, token: token, client: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

// LoadHostDefault never reads JPush credentials. No configuration disables push.
func LoadHostDefault() (*HTTPS, error) {
	endpoint, token := os.Getenv("CMR_NOTIFY_URL"), os.Getenv("CMR_NOTIFY_TOKEN")
	if endpoint == "" && token == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return nil, errors.New("cannot locate notification configuration")
		}
		if runtime.GOOS == "windows" {
			return nil, nil
		} // Use process environment until native ACL verification is available.
		path := filepath.Join(dir, "claude-mobile-remote", "notifications.json")
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 16384 {
			return nil, errors.New("notification configuration requires a private regular file (chmod 600), at most 16 KiB")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, errors.New("cannot read notification configuration")
		}
		var cfg struct {
			URL   string `json:"url"`
			Token string `json:"token"`
		}
		if json.Unmarshal(data, &cfg) != nil {
			return nil, errors.New("invalid notification configuration")
		}
		endpoint, token = cfg.URL, cfg.Token
	}
	return NewHTTPS(endpoint, token)
}

func (s *HTTPS) Send(ctx context.Context, n Notification) error {
	now := time.Now()
	expiry := now.Add(EventTTL)
	if !n.ExpiresAt.IsZero() && n.ExpiresAt.Before(expiry) {
		expiry = n.ExpiresAt
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return errors.New("cannot allocate notification ID")
	}
	e := Event{EventID: hex.EncodeToString(id), Topic: n.Topic, Kind: n.Kind, ExpiresAt: expiry}
	if !e.valid(now) {
		return errors.New("invalid notification event")
	}
	data, _ := json.Marshal(e)
	lastStatus := 0
	for attempt := 0; attempt < 2; attempt++ {
		if !e.ExpiresAt.After(time.Now()) {
			return errors.New("notification expired")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(data))
		if err != nil {
			return errors.New("cannot create notification request")
		}
		req.Header.Set("Authorization", "Bearer "+s.token)
		req.Header.Set("Content-Type", "application/json")
		response, err := s.client.Do(req)
		retry := err != nil
		if response != nil {
			lastStatus = response.StatusCode
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 2048))
			response.Body.Close()
			if err == nil && response.StatusCode == http.StatusNoContent {
				return nil
			}
			retry = response.StatusCode == 429 || response.StatusCode >= 500
		}
		if !retry || attempt == 1 {
			break
		}
		select {
		case <-ctx.Done():
			return errors.New("notification request cancelled")
		case <-time.After(500 * time.Millisecond):
		}
	}
	if lastStatus != 0 {
		return fmt.Errorf("notification service returned HTTP %d", lastStatus)
	}
	return errors.New("notification service request failed")
}
