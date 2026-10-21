package transport

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tea4go/claude-mobile-remote/protocol"
)

func testAttachment() protocol.Attachment {
	return protocol.Attachment{Name: "photo.png", MediaType: "image/png", Data: base64.StdEncoding.EncodeToString([]byte("image fixture"))}
}

func TestAttachmentStagingPrivacyCleanupAndRetention(t *testing.T) {
	root := filepath.Join(t.TempDir(), "cache")
	prompt, cleanup, err := stageAttachments(root, "describe", []protocol.Attachment{testAttachment()})
	if err != nil {
		t.Fatal(err)
	}
	dirs, _ := os.ReadDir(root)
	path := filepath.Join(root, dirs[0].Name(), "attachment-1.png")
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "image fixture" || !strings.Contains(prompt, path) || !strings.Contains(prompt, "describe") {
		t.Fatal("attachment not passed as an existing private file")
	}
	fileInfo, _ := os.Stat(path)
	dirInfo, _ := os.Stat(filepath.Dir(path))
	if fileInfo.Mode().Perm() != 0600 || dirInfo.Mode().Perm() != 0700 {
		t.Fatal("unsafe permissions")
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("rejected attachment retained")
	}
	old := filepath.Join(root, "old")
	_ = os.Mkdir(old, 0700)
	ago := time.Now().Add(-8 * 24 * time.Hour)
	_ = os.Chtimes(old, ago, ago)
	_, cleanup, err = stageAttachments(root, "", []protocol.Attachment{testAttachment()})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("expired cache retained")
	}
}

func TestAttachmentRejectsMalformedAndOversizeData(t *testing.T) {
	for _, change := range []func(*protocol.Attachment){
		func(a *protocol.Attachment) { a.Name = "../secret" },
		func(a *protocol.Attachment) { a.Name = "bad\nname" },
		func(a *protocol.Attachment) { a.Data = "invalid!" },
		func(a *protocol.Attachment) { a.Data = "" },
		func(a *protocol.Attachment) {
			a.Data = base64.StdEncoding.EncodeToString(make([]byte, protocol.MaxAttachmentBytes+1))
		},
	} {
		item := testAttachment()
		change(&item)
		if _, _, err := stageAttachments(filepath.Join(t.TempDir(), "cache"), "", []protocol.Attachment{item}); err == nil {
			t.Fatal("invalid file accepted")
		}
	}
	big := testAttachment()
	big.Data = base64.StdEncoding.EncodeToString(make([]byte, protocol.MaxAttachmentBytes))
	if _, err := protocol.DecodeAttachments([]protocol.Attachment{big, big, big}); err == nil {
		t.Fatal("total limit bypassed")
	}
	if _, err := protocol.DecodeAttachments(make([]protocol.Attachment, 5)); err == nil {
		t.Fatal("count limit bypassed")
	}
	if err := validateCommand(CommandPayload{Kind: CommandSendText, Text: "x", Attachments: []protocol.Attachment{testAttachment()}}); err == nil {
		t.Fatal("attachments silently accepted on text command")
	}
}

func TestWebSocketAttachmentsUseExistingSessionQueue(t *testing.T) {
	large := testAttachment()
	large.Data = base64.StdEncoding.EncodeToString([]byte(strings.Repeat("image-bytes", 200000)))
	t.Setenv("TMPDIR", t.TempDir())
	fixture := newServerFixture(t, nil)
	socket := fixture.dial(t)
	authenticateAndSubscribe(t, fixture, socket)
	var got protocol.Input
	fixture.controller.submit = func(input protocol.Input) (string, error) { got = input; return "upload-request", nil }
	writeTestEnvelope(t, socket, TypeCommand, "session-1", "upload", "", 0, CommandPayload{Kind: CommandSendAttachments, Text: "describe", Attachments: []protocol.Attachment{large}})
	_, result := readCommandResult(t, socket, "upload")
	if !result.Accepted || got.SessionID != "session-1" || got.Source != protocol.InputMobile || !strings.Contains(got.Text, "attachment-1.png") {
		t.Fatal("file did not reach original authorized queue")
	}
	fixture.controller.submit = func(protocol.Input) (string, error) { return "", protocol.ErrQueueFull }
	root := filepath.Join(os.TempDir(), "cmr-attachments")
	dirs, _ := os.ReadDir(root)
	content, err := os.ReadFile(filepath.Join(root, dirs[0].Name(), "attachment-1.png"))
	if err != nil || base64.StdEncoding.EncodeToString(content) != large.Data {
		t.Fatal("large attachment bytes changed during websocket transport")
	}
	before, _ := os.ReadDir(root)
	writeTestEnvelope(t, socket, TypeCommand, "session-1", "rejected-upload", "", 0, CommandPayload{Kind: CommandSendAttachments, Attachments: []protocol.Attachment{testAttachment()}})
	_, result = readCommandResult(t, socket, "rejected-upload")
	after, _ := os.ReadDir(root)
	if result.Accepted || len(before) != len(after) {
		t.Fatal("rejected upload leaked files")
	}
}

func TestWebSocketAttachmentsCannotBypassPairingOrSession(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	fixture := newServerFixture(t, nil)
	socket := fixture.dial(t)
	writeTestEnvelope(t, socket, TypeCommand, "session-1", "unauthenticated", "", 0, CommandPayload{Kind: CommandSendAttachments, Attachments: []protocol.Attachment{testAttachment()}})
	if response := readTestEnvelope(t, socket); response.Type != TypeError {
		t.Fatal("unauthenticated attachment accepted")
	}
	socket = fixture.dial(t)
	authenticateAndSubscribe(t, fixture, socket)
	writeTestEnvelope(t, socket, TypeCommand, "session-other", "wrong-session", "", 0, CommandPayload{Kind: CommandSendAttachments, Attachments: []protocol.Attachment{testAttachment()}})
	if response := readTestEnvelope(t, socket); response.Type != TypeError {
		t.Fatal("cross-session attachment accepted")
	}
	if _, err := os.Stat(filepath.Join(os.TempDir(), "cmr-attachments")); !os.IsNotExist(err) {
		t.Fatal("unauthorized request wrote files")
	}
}
