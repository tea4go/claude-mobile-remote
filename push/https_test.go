package push

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestHTTPSReportsGatewayStatusWithoutBody(t *testing.T) {
	sender, err := NewHTTPS("https://edo.invalid/v1/events", "fixture-notification-service-token-0000")
	if err != nil {
		t.Fatal(err)
	}
	sender.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("private upstream detail"))}, nil
	})}
	err = sender.Send(context.Background(), Notification{Topic: Topic("s", "synthetic-pairing"), SessionID: "s", Kind: "approval", ExpiresAt: time.Now().Add(time.Minute)})
	if err == nil || !strings.Contains(err.Error(), "HTTP 502") || strings.Contains(err.Error(), "private") {
		t.Fatal("unsafe or missing gateway status")
	}
}
