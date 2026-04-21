package tserver

import (
	"io"
	"net/http"

	"github.com/choveylee/tserver/middleware"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// NewRouter returns a Gin [gin.Engine] configured with panic recovery, OpenTelemetry HTTP
// instrumentation for serviceName, structured access logging (discarded by default), request
// latency metrics, request-body reuse middleware, and a /healthz probe.
func NewRouter(serviceName string) *gin.Engine {
	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(otelgin.Middleware(serviceName))
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: logFormatter,
		Output:    io.Discard,
	}))
	router.Use(ginMetric())
	router.Use(tmiddleware.ReuseMiddleware())

	// health check
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
		})
	})

	return router
}
