/**
 * @Author: lidonglin
 * @Description:
 * @File:  reuse.go
 * @Version: 1.0.0
 * @Date: 2024/02/28 15:46
 */

package tmiddleware

import (
	"bytes"
	"io"

	"github.com/gin-gonic/gin"
)

func ReuseMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		buf := bytes.Buffer{}
		c.Request.Body = io.NopCloser(io.TeeReader(c.Request.Body, &buf))

		c.Next()

		c.Request.Body = io.NopCloser(&buf)
	}
}
