package tserver

import (
	"context"
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
		"end-to-end latency",
		[]string{"http_method", "http_server_route", "http_status"},
	)
	if err != nil {
		tlog.E(context.Background()).Err(err).Msg("Failed to initialize HTTP server latency metrics")
	}
}

// ginMetric returns middleware that records request latency in httpServerLatency after the
// handler chain completes, using the registered route template when available.
func ginMetric() gin.HandlerFunc {
	return func(c *gin.Context) {
		if httpServerLatency == nil {
			c.Next()

			return
		}

		startTime := time.Now()

		c.Next()

		method := c.Request.Method
		status := c.Writer.Status()

		// Use the registered route pattern only (e.g. /users/:id). Never label with URL.Path:
		// unbounded paths (/v1/items/1, /v1/items/2, ...) explode Prometheus cardinality.
		route := c.FullPath()
		if route == "" {
			route = routeLabelUnknown
		}

		httpServerLatency.Observe(tmetric.SinceMS(startTime), method, route, strconv.Itoa(status))
	}
}
