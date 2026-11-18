package tserver

import (
	"net/http"

	"github.com/choveylee/tserver/middleware"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// NewRouter returns a [gin.Engine] preconfigured with panic recovery, OpenTelemetry HTTP
// instrumentation for serviceName, structured access logging, request-latency
// metrics, request-body reuse and a /healthz probe. SetTrustedProxies can opt in
// to specific reverse proxies for forwarded client IPs.
func NewRouter(serviceName string) *gin.Engine {
	router := gin.New()
	_ = router.SetTrustedProxies(nil)

	router.Use(
		prepareHTTPTrace,
		otelgin.Middleware(serviceName),
		serveTracedRequest,
		logHTTPAccess,
		ginMetric(),
		httpRecovery,
		tmiddleware.ReuseMiddleware(),
	)

	// Health check endpoint.
	router.GET("/healthz", func(ginContext *gin.Context) {
		ginContext.JSON(http.StatusOK, gin.H{
			"code": 0,
		})
	})

	return router
}
