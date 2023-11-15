/**
 * @Author: lidonglin
 * @Description:
 * @File:  middleware.go
 * @Version: 1.0.0
 * @Date: 2023/11/15 14:07
 */

package tserver

import (
	"bytes"
	"io"

	"github.com/gin-gonic/gin"
)

func reuseMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		buf := bytes.Buffer{}
		c.Request.Body = io.NopCloser(io.TeeReader(c.Request.Body, &buf))

		c.Next()

		c.Request.Body = io.NopCloser(&buf)
	}
}
