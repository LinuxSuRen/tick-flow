package integration

import (
	"io"
	"mime/multipart"
	"net/textproto"
)

// newMultipartWriter builds a single-file multipart body the way Gin's
// c.FormFile expects.
func newMultipartWriter(w io.Writer, field, filename, contentType string, content []byte) *multipart.Writer {
	writer := multipart.NewWriter(w)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		`form-data; name="`+field+`"; filename="`+filename+`"`)
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	part, _ := writer.CreatePart(h)
	_, _ = part.Write(content)
	_ = writer.Close()
	return writer
}
