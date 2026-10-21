package push

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestJPushSendsOnlyScopedPrivateReminder(t *testing.T) {
	sender, err := newJPush("fixture-key", "fixture-secret", false)
	if err != nil {
		t.Fatal(err)
	}
	topic := Topic("s", "synthetic-pairing")
	if len(topic) != 36 || topic == Topic("other", "synthetic-pairing") || topic == Topic("s", "different-pairing") {
		t.Fatal("topic is not session/credential scoped")
	}
	sender.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		key, secret, ok := r.BasicAuth()
		if !ok || key != "fixture-key" || secret != "fixture-secret" || r.URL.String() != "https://api.jpush.cn/v3/push" {
			t.Fatal("wrong authentication/destination")
		}
		data, _ := io.ReadAll(r.Body)
		if strings.Contains(string(data), "synthetic-pairing") || strings.Contains(string(data), "fixture-secret") {
			t.Fatal("credential leaked into body")
		}
		var payload struct {
			Audience struct {
				Tag []string `json:"tag"`
			} `json:"audience"`
			Notification struct {
				IOS struct {
					Extras struct {
						SessionID string `json:"sessionId"`
						Seq       uint64 `json:"seq"`
					} `json:"extras"`
				} `json:"ios"`
			} `json:"notification"`
			Options struct {
				Production bool `json:"apns_production"`
			} `json:"options"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Audience.Tag) != 1 || payload.Audience.Tag[0] != topic || payload.Notification.IOS.Extras.SessionID != "s" || payload.Notification.IOS.Extras.Seq != 7 || payload.Options.Production {
			t.Fatal("wrong reminder scope")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"msg_id":1}`))}, nil
	})}
	if err := sender.Send(context.Background(), Notification{Topic: topic, SessionID: "s", Seq: 7, Kind: "completed"}); err != nil {
		t.Fatal(err)
	}
}

func TestJPushDoesNotReturnProviderBody(t *testing.T) {
	sender, _ := newJPush("fixture-key", "fixture-secret", true)
	sender.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader("private upstream detail"))}, nil
	})}
	err := sender.Send(context.Background(), Notification{Topic: Topic("s", "synthetic-pairing"), SessionID: "s", Seq: 1, Kind: "failed"})
	if err == nil || strings.Contains(err.Error(), "private") {
		t.Fatal("unsafe or missing error")
	}
}
