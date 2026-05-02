package tmiddleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

const defaultReuseBodyLimit int64 = 1 << 20

// ReuseMiddleware snapshots request bodies up to [defaultReuseBodyLimit] bytes before the
// handler chain runs. Small bodies can then be read again later in the chain after a full
// read to EOF, while larger bodies stay streamable for the handler chain and still leave a
// bounded snapshot available for post-handler consumers such as access logging.
func ReuseMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request == nil || c.Request.Body == nil || c.Request.Body == http.NoBody {
			c.Next()

			return
		}

		snapshot, err := io.ReadAll(io.LimitReader(c.Request.Body, defaultReuseBodyLimit+1))
		if err != nil {
			c.Error(err)
			c.Request.Body = newPrefixThenErrorBody(snapshot, err)

			c.Next()

			c.Request.Body = newReusableBody(c.Request, snapshot)

			return
		}

		if len(snapshot) <= int(defaultReuseBodyLimit) {
			c.Request.Body = newReusableBody(c.Request, snapshot)

			c.Next()

			c.Request.Body = newReusableBody(c.Request, snapshot)

			return
		}

		c.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(snapshot), c.Request.Body))

		c.Next()

		c.Request.Body = newReusableBody(c.Request, snapshot)
	}
}

type reusableBody struct {
	request *http.Request
	data    []byte
	reader  *bytes.Reader
	closed  bool
}

func newReusableBody(request *http.Request, data []byte) io.ReadCloser {
	return &reusableBody{
		request: request,
		data:    data,
		reader:  bytes.NewReader(data),
	}
}

func (body *reusableBody) Read(p []byte) (int, error) {
	if body.closed {
		return 0, http.ErrBodyReadAfterClose
	}

	n, err := body.reader.Read(p)
	if err == io.EOF && body.request != nil && body.request.Body == body {
		body.request.Body = newReusableBody(body.request, body.data)
	}

	return n, err
}

func (body *reusableBody) Close() error {
	body.closed = true

	return nil
}

type prefixThenErrorBody struct {
	reader *bytes.Reader
	err    error
	closed bool
}

func newPrefixThenErrorBody(prefix []byte, err error) io.ReadCloser {
	return &prefixThenErrorBody{
		reader: bytes.NewReader(prefix),
		err:    err,
	}
}

func (body *prefixThenErrorBody) Read(p []byte) (int, error) {
	if body.closed {
		return 0, http.ErrBodyReadAfterClose
	}

	n, err := body.reader.Read(p)
	if err == io.EOF {
		return n, body.err
	}

	return n, err
}

func (body *prefixThenErrorBody) Close() error {
	body.closed = true

	return nil
}
