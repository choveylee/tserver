package tmiddleware

import (
	"bytes"
	"io"

	"github.com/gin-gonic/gin"
)

// ReuseMiddleware buffers the request body while the handler chain runs, then restores the body
// so later stages (or subsequent reads) can consume the same payload.
func ReuseMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		buf := bytes.Buffer{}
		c.Request.Body = io.NopCloser(io.TeeReader(c.Request.Body, &buf))

		c.Next()

		c.Request.Body = io.NopCloser(&buf)
	}
}
