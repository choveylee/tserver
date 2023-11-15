/**
 * @Author: lidonglin
 * @Description:
 * @File:  http_metric.go
 * @Version: 1.0.0
 * @Date: 2023/11/15 14:02
 */

package tserver

import (
	"strconv"
	"time"

	"github.com/choveylee/tmetric"
	"github.com/gin-gonic/gin"
)

var httpServerLatency *tmetric.HistogramVec

func ginMetric() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		c.Next()

		method := c.Request.Method
		status := c.Writer.Status()

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		httpServerLatency.Observe(tmetric.SinceMS(startTime), method, path, strconv.Itoa(status))
	}
}
