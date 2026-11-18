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
			replay := &bodyReplay{request: request, ginContext: ginContext}
			body := &capturingBody{ReadCloser: request.Body, replay: replay}

			request.Body = body
			defer func() {
				replay.request = ginContext.Request
				replay.ginContext = nil
			}()
		}

		ginContext.Next()
	}
}

type bodyReplay struct {
	request    *http.Request
	ginContext *gin.Context
}

// reset installs a fresh reader on the active request. Middleware can replace
// the request via WithContext or Clone while keeping the same body.
func (replay *bodyReplay) reset(current io.ReadCloser, snapshot []byte, release func() error) {
	request := replay.request
	if replay.ginContext != nil {
		request = replay.ginContext.Request
	}

	if request != nil && request.Body == current {
		body := &reusableBody{replay: replay, data: snapshot, reader: bytes.NewReader(snapshot), release: release}

		request.Body = body
	}
}

type capturingBody struct {
	io.ReadCloser

	replay       *bodyReplay
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
	if err == io.EOF {
		body.replay.reset(body, body.snapshot, body.Close)
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
	replay  *bodyReplay
	data    []byte
	reader  *bytes.Reader
	closed  atomic.Bool
	release func() error
}

// Read installs another reader only when the replay has been fully consumed.
func (body *reusableBody) Read(destination []byte) (int, error) {
	if body.closed.Load() {
		return 0, http.ErrBodyReadAfterClose
	}

	count, err := body.reader.Read(destination)
	if err == io.EOF {
		body.replay.reset(body, body.data, body.release)
	}

	return count, err
}

func (body *reusableBody) Close() error {
	body.closed.Store(true)

	return body.release()
}
