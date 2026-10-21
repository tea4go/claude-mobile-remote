// Package push sends privacy-preserving system notifications independently of
// mobile WebSocket lifetimes. It never receives Claude text or file content.
package push

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tea4go/claude-mobile-remote/protocol"
)

// Topic is a notification-only capability. It is not a Remote credential.
// The NUL separators and domain prefix must match the iOS implementation.
func Topic(sessionID, token string) string {
	sum := sha256.Sum256([]byte("cmr-push-v1\x00" + sessionID + "\x00" + token))
	return "cmr_" + hex.EncodeToString(sum[:16])
}

type Notification struct {
	Topic, SessionID, Kind string
	Seq                    uint64
}
type Sender interface {
	Send(context.Context, Notification) error
}

type JPush struct {
	AppKey       string `json:"appKey"`
	MasterSecret string `json:"masterSecret"`
	Production   bool   `json:"production"`
	client       *http.Client
	endpoint     string
}

// LoadDefault reads host-only credentials. Missing configuration disables push;
// malformed/partial configuration is reported without exposing its contents.
func LoadDefault() (*JPush, error) {
	key, secret := os.Getenv("CMR_JPUSH_APP_KEY"), os.Getenv("CMR_JPUSH_MASTER_SECRET")
	if key != "" || secret != "" {
		production := os.Getenv("CMR_JPUSH_PRODUCTION")
		if production != "" && production != "true" && production != "false" {
			return nil, errors.New("CMR_JPUSH_PRODUCTION must be true or false")
		}
		return newJPush(key, secret, production == "true")
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "claude-mobile-remote", "jpush.json")
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("cannot inspect JPush configuration")
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 16384 {
		return nil, errors.New("JPush configuration must be a private regular file (chmod 600), at most 16 KiB")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("cannot read JPush configuration")
	}
	var cfg JPush
	if json.Unmarshal(data, &cfg) != nil {
		return nil, errors.New("invalid JPush configuration JSON")
	}
	return newJPush(cfg.AppKey, cfg.MasterSecret, cfg.Production)
}

func newJPush(key, secret string, production bool) (*JPush, error) {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(secret) == "" || strings.ContainsAny(key+secret, "\r\n:") {
		return nil, errors.New("JPush requires valid appKey and masterSecret")
	}
	return &JPush{AppKey: key, MasterSecret: secret, Production: production,
		endpoint: "https://api.jpush.cn/v3/push", client: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func Kind(event protocol.Event) string {
	switch event.Kind {
	case protocol.EventActionRequired:
		if event.PendingAction != nil && event.PendingAction.Kind == protocol.ActionAskUserQuestion {
			return "question"
		}
		return "approval"
	case protocol.EventTurnCompleted:
		if event.TurnStatus != protocol.TurnCancelled {
			return "completed"
		}
	case protocol.EventTurnFailed, protocol.EventSessionFailed:
		return "failed"
	case protocol.EventSessionStopped:
		return "stopped"
	}
	return ""
}

func (j *JPush) Send(ctx context.Context, n Notification) error {
	body := map[string]string{"approval": "有工具操作等待审批", "question": "有问题等待回答", "completed": "Claude 已完成本轮任务", "failed": "Claude 执行失败，请打开会话查看", "stopped": "Claude 会话已结束"}[n.Kind]
	if body == "" || !strings.HasPrefix(n.Topic, "cmr_") || len(n.Topic) != 36 || n.SessionID == "" || n.Seq == 0 {
		return errors.New("invalid push notification")
	}
	payload := map[string]any{
		"platform": []string{"ios"}, "audience": map[string]any{"tag": []string{n.Topic}},
		"notification": map[string]any{"ios": map[string]any{"alert": map[string]string{"title": "Claude Mobile Remote", "body": body}, "sound": "default", "thread-id": n.Topic,
			"extras": map[string]any{"sessionId": n.SessionID, "seq": n.Seq, "kind": n.Kind}}},
		"options": map[string]any{"time_to_live": 86400, "apns_production": j.Production, "apns_collapse_id": n.Topic[4:]},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return errors.New("cannot encode push notification")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, j.endpoint, bytes.NewReader(data))
	if err != nil {
		return errors.New("cannot create push request")
	}
	req.SetBasicAuth(j.AppKey, j.MasterSecret)
	req.Header.Set("Content-Type", "application/json")
	response, err := j.client.Do(req)
	if err != nil {
		return errors.New("JPush request failed")
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 16384))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("JPush returned HTTP %d", response.StatusCode)
	}
	return nil
}
