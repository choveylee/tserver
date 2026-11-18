package tmiddleware

import (
	"bytes"
	"io"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gin-gonic/gin"
)

const defaultReuseBodyLimit = 1 << 20

// ReuseMiddleware captures up to 1 MiB as the handler reads. A complete body
// within the limit can be read again once the reader reaches EOF.
func ReuseMiddleware() gin.HandlerFunc {
	return func(ginContext *gin.Context) {
		request := ginContext.Request
		if request != nil && request.Body != nil && request.Body != http.NoBody && request.ContentLength <= defaultReuseBodyLimit {
			body := &capturingBody{ReadCloser: request.Body, request: request}

			request.Body = body
		}

		ginContext.Next()
	}
}

type capturingBody struct {
	io.ReadCloser

	request      *http.Request
	snapshot     []byte
	snapshotLost bool
	closeOnce    sync.Once
	closeErr     error
}

// Read captures bytes only as the underlying body supplies them. A partial,
// failed or oversized body must never look like a complete replay.
func (body *capturingBody) Read(destination []byte) (int, error) {
	count, err := body.ReadCloser.Read(destination)
	if body.snapshotLost {
		return count, err
	}

	if count > defaultReuseBodyLimit-len(body.snapshot) || (err != nil && err != io.EOF) {
		body.snapshotLost = true
		body.snapshot = nil

		return count, err
	}

	body.snapshot = append(body.snapshot, destination[:count]...)
	if err == io.EOF && body.request.Body == body {
		body.request.Body = newReusableBody(body.request, body.snapshot, body.Close)
		body.snapshotLost = true
		body.snapshot = nil
	}

	return count, err
}

// Close releases the original body, including when a replay is closed instead.
func (body *capturingBody) Close() error {
	body.closeOnce.Do(func() { body.closeErr = body.ReadCloser.Close() })

	return body.closeErr
}

type reusableBody struct {
	request *http.Request
	data    []byte
	reader  *bytes.Reader
	closed  atomic.Bool
	release func() error
}

func newReusableBody(request *http.Request, data []byte, release func() error) *reusableBody {
	body := &reusableBody{request: request, data: data, reader: bytes.NewReader(data), release: release}

	return body
}

// Read installs another reader only when the replay has been fully consumed.
func (body *reusableBody) Read(destination []byte) (int, error) {
	if body.closed.Load() {
		return 0, http.ErrBodyReadAfterClose
	}

	count, err := body.reader.Read(destination)
	if err == io.EOF && body.request.Body == body {
		body.request.Body = newReusableBody(body.request, body.data, body.release)
	}

	return count, err
}

func (body *reusableBody) Close() error {
	body.closed.Store(true)

	return body.release()
}
