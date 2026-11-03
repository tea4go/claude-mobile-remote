package protocol

import (
	"encoding/base64"
	"fmt"
	"net/http"
)

// DisplayImages validates image data before it is exposed to paired clients.
// Invalid or oversized images are omitted; the returned count lets the host
// show a placeholder without leaking the original data or a local path.
func DisplayImages(items []Attachment) ([]Attachment, int) {
	capacity := len(items)
	if capacity > MaxAttachments {
		capacity = MaxAttachments
	}
	result := make([]Attachment, 0, capacity)
	omitted, total := 0, 0
	for _, item := range items {
		if len(result) >= MaxAttachments || len(item.Data) > base64.StdEncoding.EncodedLen(MaxAttachmentBytes) {
			omitted++
			continue
		}
		data, err := base64.StdEncoding.Strict().DecodeString(item.Data)
		if err != nil || len(data) == 0 || len(data) > MaxAttachmentBytes || total+len(data) > MaxAttachmentsBytes {
			omitted++
			continue
		}
		mediaType := http.DetectContentType(data)
		if mediaType != item.MediaType || (mediaType != "image/jpeg" && mediaType != "image/png" && mediaType != "image/gif" && mediaType != "image/webp") {
			omitted++
			continue
		}
		total += len(data)
		result = append(result, Attachment{Name: fmt.Sprintf("image-%d", len(result)+1), MediaType: mediaType, Data: item.Data})
	}
	return result, omitted
}
