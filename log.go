package tserver

import (
	"io"
	"net/http"

	"github.com/choveylee/tlog"
	"github.com/gin-gonic/gin"
)

// maxLogLen caps how much of the request body is read for error access logs to limit
// memory use and log volume. One extra byte is read to detect truncation.
const maxLogLen = 1024

// logFormatter implements [gin.LoggerConfig.Formatter]: it emits a structured access log via tlog
// with method, latency, status, path, client IP, and optional query, error, and body details.
// On client-error responses, request body is logged up to [maxLogLen], with a suffix if truncated.
func logFormatter(param gin.LogFormatterParams) string {
	event := tlog.D(param.Request.Context()).
		Detailf("method:%s", param.Method).
		Detailf("latency:%v", param.Latency).
		Detailf("code:%d", param.StatusCode).
		Detailf("path:%s", param.Path).
		Detailf("client_ip:%s", param.ClientIP).
		Detailf("response_size:%d", param.BodySize)

	if len(param.Request.URL.RawQuery) > 0 {
		event.Detailf("query:%s", param.Request.URL.RawQuery)
	}

	if len(param.ErrorMessage) > 0 {
		event.Detailf("error:%s", param.ErrorMessage)
	}

	if (param.Method == http.MethodPost || param.Method == http.MethodPut ||
		param.Method == http.MethodPatch || param.Method == http.MethodDelete) &&
		param.StatusCode >= http.StatusBadRequest {
		limited := io.LimitReader(param.Request.Body, maxLogLen+1)

		body, err := io.ReadAll(limited)
		if err != nil {
			event.Detailf("body_read_error:%v", err)
		} else {
			truncated := len(body) > maxLogLen

			if truncated {
				body = body[:maxLogLen]
			}

			if len(body) == 0 {
				body = []byte("empty")
			}

			strBody := string(body)

			if truncated {
				strBody += "...(truncated)"
			}

			event.Detailf("body:%s", strBody)
		}
	}

	event.Msg("http access log.")

	return ""
}
