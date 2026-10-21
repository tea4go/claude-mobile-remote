package protocol

import (
	"encoding/base64"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxAttachments = 4
const MaxAttachmentBytes = 4 * 1024 * 1024
const MaxAttachmentsBytes = 8 * 1024 * 1024
const MaxAttachmentWireBytes = MaxAttachmentsBytes*4/3 + MaxPromptBytes + 128*1024

type Attachment struct {
	Name      string `json:"name"`
	MediaType string `json:"mediaType"`
	Data      string `json:"data"`
}

func DecodeAttachments(items []Attachment) ([][]byte, error) {
	if len(items) == 0 || len(items) > MaxAttachments {
		return nil, errors.New("invalid attachment count")
	}
	result := make([][]byte, 0, len(items))
	total := 0
	for _, item := range items {
		if !utf8.ValidString(item.Name) || len(item.Name) == 0 || len(item.Name) > 255 || item.Name == "." || item.Name == ".." || strings.ContainsAny(item.Name, "/\\") || strings.IndexFunc(item.Name, unicode.IsControl) >= 0 || len(item.MediaType) > 128 || strings.IndexFunc(item.MediaType, unicode.IsControl) >= 0 || len(item.Data) > base64.StdEncoding.EncodedLen(MaxAttachmentBytes) {
			return nil, errors.New("invalid attachment")
		}
		data, err := base64.StdEncoding.Strict().DecodeString(item.Data)
		if err != nil || len(data) == 0 || len(data) > MaxAttachmentBytes {
			return nil, errors.New("invalid attachment data")
		}
		total += len(data)
		if total > MaxAttachmentsBytes {
			return nil, errors.New("attachments too large")
		}
		result = append(result, data)
	}
	return result, nil
}
