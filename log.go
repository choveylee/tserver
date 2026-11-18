package tserver

import (
	"net/http"
	"time"

	"github.com/choveylee/tlog"
	"github.com/gin-gonic/gin"
)

// logHTTPAccess records route-level metadata after recovery and the handler chain.
func logHTTPAccess(ginContext *gin.Context) {
	startedAt := time.Now()
	ginContext.Next()

	tlog.D(ginContext.Request.Context()).
		Detailf("method:%s", httpMethodLabel(ginContext.Request.Method)).
		Detailf("latency:%v", time.Since(startedAt)).
		Detailf("code:%d", ginContext.Writer.Status()).
		Detailf("path:%s", httpRouteLabel(ginContext)).
		Detailf("client_ip:%s", ginContext.ClientIP()).
		Detailf("response_size:%d", ginContext.Writer.Size()).
		Msg("HTTP access log entry")
}

// recoverHTTP returns a generic 500 and logs metadata without dumping the request or panic value.
func recoverHTTP(ginContext *gin.Context, recovered any) {
	tlog.E(ginContext.Request.Context()).
		Detailf("route:%s", httpRouteLabel(ginContext)).
		Detailf("panic_type:%T", recovered).
		Msg("Recover HTTP handler panic")
	ginContext.AbortWithStatus(http.StatusInternalServerError)
}
