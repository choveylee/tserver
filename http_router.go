/**
 * @Author: lidonglin
 * @Description:
 * @File:  http_router.go
 * @Version: 1.0.0
 * @Date: 2023/11/15 13:59
 */

package tserver

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func NewRouter(serviceName string) *gin.Engine {
	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(otelgin.Middleware(serviceName))
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: logFormatter,
		Output:    io.Discard,
	}))
	router.Use(ginMetric())
	router.Use(reuseMiddleware())

	// health check
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
		})
	})

	return router
}
