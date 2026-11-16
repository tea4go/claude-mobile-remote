package transport

import (
	"encoding/base64"
	"encoding/json"
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
	prompt, paths, cleanup, err := stageAttachments(root, "describe", []protocol.Attachment{testAttachment()})
	if err != nil {
		t.Fatal(err)
	}
	dirs, _ := os.ReadDir(root)
	path := filepath.Join(root, dirs[0].Name(), "attachment-1.png")
	if len(paths) != 1 || paths[0] != path {
		t.Fatal("staged path missing or out of order")
	}
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
	_, _, cleanup, err = stageAttachments(root, "", []protocol.Attachment{testAttachment()})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("expired cache retained")
	}
}

func TestAttachmentStagingKeepsMixedFilesInOrder(t *testing.T) {
	items := []protocol.Attachment{testAttachment(), {Name: "notes.txt", MediaType: "text/plain", Data: base64.StdEncoding.EncodeToString([]byte("notes"))}}
	prompt, paths, cleanup, err := stageAttachments(filepath.Join(t.TempDir(), "cache"), "describe", items)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if len(paths) != len(items) || !strings.HasSuffix(paths[0], "attachment-1.png") || !strings.HasSuffix(paths[1], "attachment-2.txt") || !strings.Contains(prompt, paths[1]) {
		t.Fatalf("mixed attachments lost their ordered paths: %v", paths)
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
		if _, _, _, err := stageAttachments(filepath.Join(t.TempDir(), "cache"), "", []protocol.Attachment{item}); err == nil {
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
	if got.Display == nil || len(got.Display.StagedPaths) != 1 || !strings.Contains(got.Display.StagedPaths[0], "attachment-1.png") {
		t.Fatal("trusted attachment path was not delivered to the host")
	}
	encoded, err := json.Marshal(got.Display)
	if err != nil || strings.Contains(string(encoded), got.Display.StagedPaths[0]) {
		t.Fatal("trusted local path must not be serialized")
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

func TestStagedImageReferenceIsSessionScopedAndExpires(t *testing.T) {
	root := filepath.Join(t.TempDir(), "cache")
	items := []protocol.Attachment{testAttachment(), {Name: "notes.txt", MediaType: "text/plain", Data: base64.StdEncoding.EncodeToString([]byte("note"))}}
	_, paths, cleanup, err := stageAttachments(root, "", items)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	refs, err := registerStagedImages("session-a", paths, items)
	if err != nil || len(refs) != 2 || refs[0] == "" || refs[1] != "" {
		t.Fatalf("invalid references: %v %v", refs, err)
	}
	image, err := readStagedImage(root, "session-a", refs[0])
	if err != nil || image != items[0] {
		t.Fatalf("image mismatch: %v", err)
	}
	for _, ref := range []string{refs[0], "../secret:0", "upload-x:../0", "upload-x:4"} {
		if _, err := readStagedImage(root, "session-b", ref); err == nil {
			t.Fatal("unauthorized reference accepted")
		}
	}
	ago := time.Now().Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(filepath.Dir(paths[0]), ago, ago); err != nil {
		t.Fatal(err)
	}
	if _, err := readStagedImage(root, "session-a", refs[0]); err == nil {
		t.Fatal("expired image accepted")
	}
}

func TestImageRestorationOverVersion10WebSocket(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	fixture := newServerFixture(t, nil)
	socket := fixture.dial(t)
	writeVersionedTestEnvelope(t, socket, 10, TypeHello, "session-1", HelloPayload{PairToken: fixture.token})
	_ = readTestEnvelope(t, socket)
	writeVersionedTestEnvelope(t, socket, 10, TypeSubscribe, "session-1", struct{}{})
	_ = readTestEnvelope(t, socket)
	_ = readTestEnvelope(t, socket)
	fixture.controller.submit = func(input protocol.Input) (string, error) { return "request", nil }
	item := testAttachment()
	env, _ := NewEnvelope(TypeCommand, "session-1", "upload", "", 0, CommandPayload{Kind: CommandSendAttachments, Attachments: []protocol.Attachment{item}})
	env.Version = 10
	data, _ := EncodeEnvelope(env)
	if err := socket.WriteMessage(1, data); err != nil {
		t.Fatal(err)
	}
	_, result := readCommandResult(t, socket, "upload")
	if len(result.ImageRefs) != 1 || result.ImageRefs[0] == "" {
		t.Fatalf("missing reference: %+v", result)
	}
	writeVersionedTestEnvelope(t, socket, 10, TypeImageGet, "session-1", ImageRequestPayload{Ref: result.ImageRefs[0]})
	response := readTestEnvelope(t, socket)
	var payload ImageResultPayload
	if response.Type != TypeImageResult || DecodePayload(response, &payload) != nil || payload.Image == nil || *payload.Image != item {
		t.Fatalf("image not restored: %+v", response)
	}
}
