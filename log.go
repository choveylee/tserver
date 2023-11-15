/**
 * @Author: lidonglin
 * @Description:
 * @File:  log
 * @Version: 1.0.0
 * @Date: 2023/11/15 14:19
 */

package tserver

import (
	"io"
	"net/http"

	"github.com/choveylee/tlog"
	"github.com/gin-gonic/gin"
)

func logFormatter(param gin.LogFormatterParams) string {
	event := tlog.I(param.Request.Context()).
		Detailf("method: %s; ", param.Method).
		Detailf("latency: %v;", param.Latency).
		Detailf("code: %d;", param.StatusCode).
		Detailf("Path: %s;", param.Path).
		Detailf("client_ip: %s;", param.ClientIP).
		Detailf("response_size: %d;", param.BodySize)

	if len(param.Request.URL.RawQuery) > 0 {
		event.Detailf("query: %s;", param.Request.URL.RawQuery)
	}

	if len(param.ErrorMessage) > 0 {
		event.Detailf("error: %s;", param.ErrorMessage)
	}

	if (param.Method == http.MethodPost || param.Method == http.MethodPut ||
		param.Method == http.MethodPatch || param.Method == http.MethodDelete) &&
		param.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(param.Request.Body)

		if len(body) == 0 {
			body = []byte("empty")
		}

		event.Detailf("body: %s;", string(body))
	}

	event.Msg("http access log.")

	return ""
}
