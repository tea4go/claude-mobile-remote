package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const fixtureToken = "fixture-notification-service-token-0000"

type senderFunc func(context.Context, Notification) error

func (f senderFunc) Send(ctx context.Context, n Notification) error { return f(ctx, n) }
func fixtureEvent() Event {
	return Event{EventID: strings.Repeat("a", 32), Topic: Topic("private-session", "private-pair-token"), Kind: "approval", ExpiresAt: time.Now().Add(time.Minute).UTC()}
}
func intake(g *Gateway, data []byte, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "https://edo.invalid/v1/events", bytes.NewReader(data))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	g.ServeHTTP(w, r)
	return w
}
func TestGatewayRejectsUnauthorizedAndUnsafeEvents(t *testing.T) {
	var calls atomic.Int32
	g, _ := NewGateway(fixtureToken, senderFunc(func(context.Context, Notification) error { calls.Add(1); return nil }))
	event := fixtureEvent()
	valid, _ := json.Marshal(event)
	if w := intake(g, valid, "wrong"); w.Code != 401 {
		t.Fatal(w.Code)
	}
	cases := [][]byte{[]byte(`{"sessionId":"private"}`), append(valid, []byte(` {}`)...), []byte(strings.Repeat("x", 2049))}
	for _, mutate := range []func(*Event){func(e *Event) { e.Topic = "cmr_" + strings.Repeat("z", 32) }, func(e *Event) { e.Kind = "arbitrary message" }, func(e *Event) { e.ExpiresAt = time.Now().Add(-time.Second) }, func(e *Event) { e.ExpiresAt = time.Now().Add(time.Hour) }} {
		e := event
		mutate(&e)
		data, _ := json.Marshal(e)
		cases = append(cases, data)
	}
	for _, data := range cases {
		if w := intake(g, data, fixtureToken); w.Code != 400 {
			t.Fatalf("unsafe request accepted: %d", w.Code)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid request reached provider")
	}
}
func TestGatewayDeduplicatesConcurrentRequestsAndRejectsCollisions(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	g, _ := NewGateway(fixtureToken, senderFunc(func(_ context.Context, n Notification) error {
		calls.Add(1)
		if n.SessionID != "" || n.Seq != 0 {
			t.Error("session metadata reached gateway sender")
		}
		close(entered)
		<-release
		return nil
	}))
	event := fixtureEvent()
	data, _ := json.Marshal(event)
	done := make(chan int, 1)
	go func() { done <- intake(g, data, fixtureToken).Code }()
	<-entered
	if w := intake(g, data, fixtureToken); w.Code != 503 {
		t.Fatal(w.Code)
	}
	close(release)
	if code := <-done; code != 204 {
		t.Fatal(code)
	}
	if w := intake(g, data, fixtureToken); w.Code != 204 {
		t.Fatal(w.Code)
	}
	event.Kind = "failed"
	collision, _ := json.Marshal(event)
	if w := intake(g, collision, fixtureToken); w.Code != 409 {
		t.Fatal(w.Code)
	}
	if calls.Load() != 1 {
		t.Fatal("duplicate delivery")
	}
}
func TestGatewayFailureCanRetryWithoutLeakingProviderError(t *testing.T) {
	var calls int
	g, _ := NewGateway(fixtureToken, senderFunc(func(context.Context, Notification) error {
		calls++
		if calls == 1 {
			return errors.New("private upstream response")
		}
		return nil
	}))
	data, _ := json.Marshal(fixtureEvent())
	w := intake(g, data, fixtureToken)
	if w.Code != 502 || strings.Contains(w.Body.String(), "private") {
		t.Fatal("unsafe failure")
	}
	if w = intake(g, data, fixtureToken); w.Code != 204 || calls != 2 {
		t.Fatal("retry suppressed")
	}
}

func TestGatewayRegistersDeviceThenRoutesAndRevokesNotice(t *testing.T) {
	devices, err := LoadDeviceRegistry(filepath.Join(t.TempDir(), "devices.json"))
	if err != nil {
		t.Fatal(err)
	}
	var targets []string
	g, err := NewGatewayWithDevices(fixtureToken, senderFunc(func(_ context.Context, notice Notification) error {
		targets = append([]string(nil), notice.RegistrationIDs...)
		return nil
	}), devices)
	if err != nil {
		t.Fatal(err)
	}
	event := fixtureEvent()
	data, _ := json.Marshal(event)
	if code := intake(g, data, fixtureToken).Code; code != 404 {
		t.Fatalf("unregistered event status = %d", code)
	}
	registration, _ := json.Marshal(deviceRequest{Topic: event.Topic, RegistrationID: "010b81b3582", ExpiresAt: time.Now().Add(time.Hour)})
	deviceCall := func(method string, body []byte) int {
		req := httptest.NewRequest(method, "https://edo.invalid/v1/events", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		g.ServeHTTP(w, req)
		return w.Code
	}
	if code := deviceCall(http.MethodPut, append(registration, []byte(` {}`)...)); code != 400 {
		t.Fatalf("extra JSON accepted: %d", code)
	}
	if code := deviceCall(http.MethodPut, registration); code != 204 {
		t.Fatalf("registration status = %d", code)
	}
	if code := intake(g, data, fixtureToken).Code; code != 204 || len(targets) != 1 || targets[0] != "010b81b3582" {
		t.Fatalf("registered event = %d, targets=%v", code, targets)
	}
	remove, _ := json.Marshal(deviceRequest{Topic: event.Topic, RegistrationID: "010b81b3582"})
	if code := deviceCall(http.MethodDelete, remove); code != 204 {
		t.Fatalf("removal status = %d", code)
	}
	event.EventID = strings.Repeat("b", 32)
	data, _ = json.Marshal(event)
	if code := intake(g, data, fixtureToken).Code; code != 404 {
		t.Fatalf("removed device still targeted: %d", code)
	}
}
func TestGatewayCapacityAndRateAreBounded(t *testing.T) {
	g, _ := NewGateway(fixtureToken, senderFunc(func(context.Context, Notification) error { return nil }))
	e := fixtureEvent()
	data, _ := json.Marshal(e)
	for _, mode := range []string{"rate", "capacity", "concurrency"} {
		g.tokens = 20
		g.active = 0
		g.receipts = make(map[string]*receipt)
		switch mode {
		case "rate":
			g.tokens = 0
		case "capacity":
			for i := 0; i < 4096; i++ {
				g.receipts[strings.Repeat("x", i+1)] = &receipt{event: e, done: true}
			}
		case "concurrency":
			g.active = 8
		}
		if w := intake(g, data, fixtureToken); w.Code != 429 {
			t.Fatalf("%s not bounded: %d", mode, w.Code)
		}
	}
}
func TestHTTPSToGatewayToJPushStripsPrivateMetadata(t *testing.T) {
	j, _ := newJPush("fixture-app", "fixture-secret", true)
	j.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		data, _ := io.ReadAll(r.Body)
		for _, secret := range []string{"private-session", "private-pair-token", fixtureToken, "fixture-secret", "sessionId"} {
			if bytes.Contains(data, []byte(secret)) {
				t.Errorf("private metadata leaked: %s", secret)
			}
		}
		var payload map[string]any
		if json.Unmarshal(data, &payload) != nil {
			t.Fatal("invalid provider request")
		}
		ios := payload["notification"].(map[string]any)["ios"].(map[string]any)
		extras := ios["extras"].(map[string]any)
		audience := payload["audience"].(map[string]any)
		if _, hasTag := audience["tag"]; hasTag || audience["registration_id"].([]any)[0] != "010b81b3582" {
			t.Fatal("push used a tag or the wrong device")
		}
		if extras["topic"] != Topic("private-session", "private-pair-token") || len(extras) != 3 {
			t.Fatal("wrong route")
		}
		if payload["options"].(map[string]any)["apns_production"] != true {
			t.Fatal("wrong APNs environment")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"msg_id":1}`))}, nil
	})}
	devices, err := LoadDeviceRegistry(filepath.Join(t.TempDir(), "devices.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := devices.Register(Topic("private-session", "private-pair-token"), "010b81b3582", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	g, _ := NewGatewayWithDevices(fixtureToken, j, devices)
	s, _ := NewHTTPS("https://edo.invalid/v1/events", fixtureToken)
	s.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		g.ServeHTTP(w, r)
		return w.Result(), nil
	})}
	if err := s.Send(context.Background(), Notification{Topic: Topic("private-session", "private-pair-token"), SessionID: "private-session", Seq: 42, Kind: "approval", ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
}
func TestHTTPSRetriesSameIDAndSanitizesFailures(t *testing.T) {
	s, _ := NewHTTPS("https://edo.invalid/v1/events", fixtureToken)
	var first []byte
	calls := 0
	s.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		data, _ := io.ReadAll(r.Body)
		if calls == 1 {
			first = data
		} else if !bytes.Equal(first, data) {
			t.Error("retry changed event identity")
		}
		if r.Header.Get("Authorization") != "Bearer "+fixtureToken {
			t.Error("missing auth")
		}
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("private upstream"))}, nil
	})}
	err := s.Send(context.Background(), Notification{Topic: fixtureEvent().Topic, Kind: "question"})
	if err == nil || calls != 2 || strings.Contains(err.Error(), "private") {
		t.Fatal("retry/error contract failed")
	}
	calls = 0
	s.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader("secret"))}, nil
	})}
	_ = s.Send(context.Background(), Notification{Topic: fixtureEvent().Topic, Kind: "question"})
	if calls != 1 {
		t.Fatal("authentication failure retried")
	}
}
func TestHTTPSRequiresSafeEndpointAndStopsOnCancellation(t *testing.T) {
	for _, endpoint := range []string{"http://edo.invalid/events", "https://u:p@edo.invalid/events", "https://edo.invalid/events?token=private", "https://edo.invalid/events#secret"} {
		if _, err := NewHTTPS(endpoint, fixtureToken); err == nil {
			t.Fatal("unsafe endpoint")
		}
	}
	s, _ := NewHTTPS("https://edo.invalid/v1/events", fixtureToken)
	s.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("private connection failure") })}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Send(ctx, Notification{Topic: fixtureEvent().Topic, Kind: "failed"}); err == nil || strings.Contains(err.Error(), "private") {
		t.Fatal("unsafe cancellation")
	}
	if s2, _ := NewHTTPS("https://edo.invalid/v1/events", fixtureToken); s2.client.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
		t.Fatal("redirects enabled")
	}
}
