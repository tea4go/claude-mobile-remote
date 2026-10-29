package push

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
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
		if strings.Contains(string(data), "synthetic-pairing") || strings.Contains(string(data), "fixture-secret") || strings.Contains(string(data), "sessionId") {
			t.Fatal("credential leaked into body")
		}
		var payload struct {
			Platform []string `json:"platform"`

			Audience struct {
				Tag            []string `json:"tag"`
				RegistrationID []string `json:"registration_id"`
			} `json:"audience"`
			Notification struct {
				Android struct {
					Alert             string `json:"alert"`
					DisplayForeground string `json:"display_foreground"`
					Intent            struct {
						URL string `json:"url"`
					} `json:"intent"`
					Extras struct {
						Topic string `json:"topic"`
					} `json:"extras"`
				} `json:"android"`
				IOS struct {
					Extras struct {
						Topic   string `json:"topic"`
						EventID string `json:"eventId"`
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
		if len(payload.Audience.Tag) != 0 || len(payload.Audience.RegistrationID) != 1 || payload.Audience.RegistrationID[0] != "010b81b3582" || payload.Notification.IOS.Extras.Topic != topic || payload.Notification.IOS.Extras.EventID != strings.Repeat("a", 32) || payload.Options.Production {
			t.Fatal("wrong reminder scope")
		}
		if strings.Join(payload.Platform, ",") != "ios,android" || payload.Notification.Android.Alert != "Claude 已完成本轮任务" {
			t.Fatal("missing platform or non-fixed text")
		}
		if payload.Notification.Android.DisplayForeground != "0" || payload.Notification.Android.Extras.Topic != topic {
			t.Fatal("foreground or route policy violated")
		}
		if !strings.HasPrefix(payload.Notification.Android.Intent.URL, "intent://notification/"+topic+"#Intent;") || !strings.Contains(payload.Notification.Android.Intent.URL, "component=com.tea4go.claudemobileremote/.MainActivity;") {
			t.Fatal("notification click cannot reach scoped app route")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"msg_id":1}`))}, nil
	})}
	if err := sender.Send(context.Background(), Notification{Topic: topic, SessionID: "s", Seq: 7, Kind: "completed", RegistrationIDs: []string{"010b81b3582"}, EventID: strings.Repeat("a", 32), ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
}

func TestTopicMatchesAndroidRouteVector(t *testing.T) {
	if Topic("fixture-session", "fixture-pairing") != "cmr_c0ffbb23aa5fec6594bfcf4b6b452eb0" {
		t.Fatal("PC and Android notification routes disagree")
	}
}

func TestJPushDoesNotReturnProviderBody(t *testing.T) {
	sender, _ := newJPush("fixture-key", "fixture-secret", true)
	sender.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader(`{"error":{"code":2009,"message":"private upstream detail"}}`))}, nil
	})}
	err := sender.Send(context.Background(), Notification{Topic: Topic("s", "synthetic-pairing"), SessionID: "s", Seq: 1, Kind: "failed", RegistrationIDs: []string{"010b81b3582"}, EventID: strings.Repeat("a", 32), ExpiresAt: time.Now().Add(time.Minute)})
	if err == nil || !strings.Contains(err.Error(), "HTTP 403 (code 2009)") || strings.Contains(err.Error(), "private") {
		t.Fatal("unsafe or missing error")
	}
}
