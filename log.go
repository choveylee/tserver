package tserver

import (
	"errors"
	"net/http"
	"syscall"
	"time"

	"github.com/choveylee/tlog"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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

// httpRecovery returns a clean 500 before response commitment. Once headers
// are committed, net/http must abort the connection or HTTP/2 stream.
func httpRecovery(ginContext *gin.Context) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}

		err, ok := recovered.(error)
		connectionAborted := ok && (errors.Is(err, http.ErrAbortHandler) || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET))
		if !connectionAborted {
			tlog.E(ginContext.Request.Context()).
				Detailf("route:%s", httpRouteLabel(ginContext)).
				Detailf("panic_type:%T", recovered).
				Msg("Recover HTTP handler panic")
		}

		if connectionAborted || ginContext.Writer.Written() {
			span := trace.SpanFromContext(ginContext.Request.Context())
			span.SetStatus(codes.Error, "HTTP response aborted")

			panic(http.ErrAbortHandler)
		}

		// The failed handler may have prepared headers for a body it never wrote.
		headerMap := ginContext.Writer.Header()
		for _, name := range []string{"Content-Length", "Content-Encoding", "Content-Type", "Content-Range", "Trailer", "Transfer-Encoding"} {
			headerMap.Del(name)
		}

		ginContext.AbortWithStatus(http.StatusInternalServerError)
	}()

	ginContext.Next()
}
