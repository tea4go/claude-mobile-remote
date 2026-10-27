package push

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"
)

type receipt struct {
	event Event
	done  bool
}

// Gateway is an authenticated, bounded notification intake, not a Session relay.
type Gateway struct {
	tokenHash    [32]byte
	sender       Sender
	devices      *DeviceRegistry
	mu           sync.Mutex
	receipts     map[string]*receipt
	active       int
	tokens       float64
	refill       time.Time
	deviceTokens float64
	deviceRefill time.Time
}

func NewGateway(token string, sender Sender) (*Gateway, error) {
	return NewGatewayWithDevices(token, sender, nil)
}

func NewGatewayWithDevices(token string, sender Sender, devices *DeviceRegistry) (*Gateway, error) {
	if len(token) < 32 || len(token) > 512 || strings.ContainsAny(token, " \r\n\t") || sender == nil {
		return nil, errors.New("notification gateway requires a dedicated token and sender")
	}
	return &Gateway{tokenHash: sha256.Sum256([]byte("Bearer " + token)), sender: sender, devices: devices, receipts: make(map[string]*receipt), tokens: 20, refill: time.Now(), deviceTokens: 20, deviceRefill: time.Now()}, nil
}
func gatewayError(w http.ResponseWriter, status int) { http.Error(w, http.StatusText(status), status) }
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Path == "/healthz" && r.Method == http.MethodGet {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.URL.Path != "/v1/events" {
		gatewayError(w, 404)
		return
	}
	if (r.Method == http.MethodPut || r.Method == http.MethodDelete) && g.devices != nil {
		g.serveDevice(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		gatewayError(w, 405)
		return
	}
	candidate := sha256.Sum256([]byte(r.Header.Get("Authorization")))
	if subtle.ConstantTimeCompare(candidate[:], g.tokenHash[:]) != 1 {
		gatewayError(w, 401)
		return
	}
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if media != "application/json" {
		gatewayError(w, 415)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var e Event
	if decoder.Decode(&e) != nil || decoder.Decode(new(any)) != io.EOF || !e.valid(time.Now()) {
		gatewayError(w, 400)
		return
	}
	var registrationIDs []string
	if g.devices != nil {
		registrationIDs = g.devices.Targets(e.Topic)
		if len(registrationIDs) == 0 {
			gatewayError(w, 404)
			return
		}
	}
	now := time.Now()
	g.mu.Lock()
	for id, old := range g.receipts {
		if !old.event.ExpiresAt.After(now) && old.done {
			delete(g.receipts, id)
		}
	}
	if old := g.receipts[e.EventID]; old != nil {
		same := old.event.Topic == e.Topic && old.event.Kind == e.Kind && old.event.ExpiresAt.Equal(e.ExpiresAt)
		done := old.done
		g.mu.Unlock()
		if !same {
			gatewayError(w, 409)
		} else if done {
			w.WriteHeader(http.StatusNoContent)
		} else {
			gatewayError(w, 503)
		}
		return
	}
	g.tokens += now.Sub(g.refill).Seconds() * 2
	if g.tokens > 20 {
		g.tokens = 20
	}
	g.refill = now
	if g.tokens < 1 || g.active >= 8 || len(g.receipts) >= 4096 {
		g.mu.Unlock()
		w.Header().Set("Retry-After", "1")
		gatewayError(w, 429)
		return
	}
	g.tokens--
	g.active++
	record := &receipt{event: e}
	g.receipts[e.EventID] = record
	g.mu.Unlock()
	err := g.sender.Send(r.Context(), Notification{Topic: e.Topic, Kind: e.Kind, EventID: e.EventID, ExpiresAt: e.ExpiresAt, RegistrationIDs: registrationIDs})
	g.mu.Lock()
	g.active--
	if err != nil {
		delete(g.receipts, e.EventID)
	} else {
		record.done = true
	}
	g.mu.Unlock()
	if err != nil {
		gatewayError(w, 502)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type deviceRequest struct {
	Topic          string    `json:"topic"`
	RegistrationID string    `json:"registrationId"`
	ExpiresAt      time.Time `json:"expiresAt"`
}

func (g *Gateway) serveDevice(w http.ResponseWriter, r *http.Request) {
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if media != "application/json" {
		gatewayError(w, 415)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request deviceRequest
	if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF ||
		!topicPattern.MatchString(request.Topic) || !registrationIDPattern.MatchString(request.RegistrationID) ||
		(r.Method == http.MethodPut && !validDeviceRoute(request.Topic, request.RegistrationID, request.ExpiresAt, time.Now())) {
		gatewayError(w, 400)
		return
	}
	g.mu.Lock()
	now := time.Now()
	g.deviceTokens += now.Sub(g.deviceRefill).Seconds() * 2
	if g.deviceTokens > 20 {
		g.deviceTokens = 20
	}
	g.deviceRefill = now
	if g.deviceTokens < 1 {
		g.mu.Unlock()
		gatewayError(w, 429)
		return
	}
	g.deviceTokens--
	g.mu.Unlock()
	var err error
	if r.Method == http.MethodPut {
		err = g.devices.Register(request.Topic, request.RegistrationID, request.ExpiresAt)
	} else {
		err = g.devices.Remove(request.Topic, request.RegistrationID)
	}
	if err != nil {
		gatewayError(w, 503)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
