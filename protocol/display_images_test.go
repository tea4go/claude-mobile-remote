package protocol

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestDisplayImagesValidatesAndRemovesUntrustedNames(t *testing.T) {
	data := base64.StdEncoding.EncodeToString([]byte("GIF89a\x01\x00\x01\x00\x00\x00\x00"))
	got, omitted := DisplayImages([]Attachment{
		{Name: "/private/picture.gif", MediaType: "image/gif", Data: data},
		{Name: "fake.png", MediaType: "image/png", Data: data},
		{Name: "invalid", MediaType: "image/gif", Data: "!!!"},
	})
	if len(got) != 1 || omitted != 2 || got[0].Name != "image-1" || got[0].Data != data {
		t.Fatalf("unexpected image validation result: count=%d omitted=%d", len(got), omitted)
	}
}

func TestDisplayImagesRejectsOversizeBeforeDecoding(t *testing.T) {
	got, omitted := DisplayImages([]Attachment{{
		MediaType: "image/png", Data: strings.Repeat("A", base64.StdEncoding.EncodedLen(MaxAttachmentBytes)+4),
	}})
	if len(got) != 0 || omitted != 1 {
		t.Fatalf("count=%d omitted=%d", len(got), omitted)
	}
}

func TestDisplayImagesEnforcesCountLimit(t *testing.T) {
	data := base64.StdEncoding.EncodeToString([]byte("GIF89a\x01\x00\x01\x00\x00\x00\x00"))
	items := make([]Attachment, MaxAttachments+1)
	for i := range items {
		items[i] = Attachment{MediaType: "image/gif", Data: data}
	}
	got, omitted := DisplayImages(items)
	if len(got) != MaxAttachments || omitted != 1 {
		t.Fatalf("count=%d omitted=%d", len(got), omitted)
	}
}
