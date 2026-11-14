package service

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
)

// bytesBuffer is a minimal in-memory sink offering a Reader() for the
// storage layer. Uploads are capped at the configured size limit, so the
// memory footprint is bounded by design.
type bytesBuffer struct {
	data []byte
}

func (b *bytesBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

// Reader exposes the buffered bytes for reading.
func (b *bytesBuffer) Reader() io.Reader {
	return &byteReader{data: b.data}
}

type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// sha256Writer accumulates a hash while the payload streams through.
type sha256Writer struct {
	h hash.Hash
}

func newSHA256() *sha256Writer {
	return &sha256Writer{h: sha256.New()}
}

func (w *sha256Writer) Write(p []byte) (int, error) {
	return w.h.Write(p)
}

func (w *sha256Writer) hex() string {
	return hex.EncodeToString(w.h.Sum(nil))
}

// copyTo streams r into w exactly once.
func copyTo(w io.Writer, r io.Reader) (int64, error) {
	return io.Copy(w, r)
}
