package tserver

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/choveylee/tlog"
	"github.com/choveylee/tmetric"
	"github.com/gin-gonic/gin"
)

var httpServerLatency *tmetric.HistogramVec

// routeLabelUnknown is used when no Gin route template applies, such as for 404 responses
// or unregistered paths. Raw URL paths must not be used as label values because each
// distinct path would create a separate metric time series.
const routeLabelUnknown = "unknown"

func init() {
	var err error
	httpServerLatency, err = tmetric.NewHistogramVec(
		"http_server_request_latency",
		"HTTP handler duration in milliseconds",
		[]string{"http_method", "http_server_route", "http_status"},
	)
	if err != nil {
		tlog.E(context.Background()).Err(err).Msg("Failed to initialize HTTP server latency metrics")
	}
}

// ginMetric returns middleware that records request latency in httpServerLatency after the
// handler chain completes, using the registered route template when available.
func ginMetric() gin.HandlerFunc {
	return func(ginContext *gin.Context) {
		startTime := time.Now()
		ginContext.Next()

		if httpServerLatency != nil {
			method := httpMethodLabel(ginContext.Request.Method)
			route := httpRouteLabel(ginContext)
			status := strconv.Itoa(ginContext.Writer.Status())
			_ = httpServerLatency.Observe(tmetric.SinceMS(startTime), method, route, status)
		}
	}
}

// httpRouteLabel keeps unmatched paths out of logs, metrics and traces.
func httpRouteLabel(ginContext *gin.Context) string {
	if route := ginContext.FullPath(); route != "" {
		return route
	}

	return routeLabelUnknown
}

// httpMethodLabel bounds telemetry cardinality for arbitrary HTTP methods.
func httpMethodLabel(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace:
		return method
	}

	return "_OTHER"
}
