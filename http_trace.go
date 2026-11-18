package tserver

import (
	"errors"
	"net"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

type httpTraceKey struct{}

type httpTraceState struct {
	request       *http.Request
	telemetry     *http.Request
	restoreErrors func()
}

// prepareHTTPTrace supplies route-level request metadata to otelgin. The
// application chain receives the original request with the trace context.
func prepareHTTPTrace(ginContext *gin.Context) {
	request := ginContext.Request
	telemetry := new(http.Request)
	*telemetry = *request
	telemetry.Method = httpMethodLabel(request.Method)
	// otelgin uses Host for server.address and server.port. The local address
	// keeps those dimensions independent of client-supplied header values.
	telemetry.Host = ""
	if address, ok := request.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
		telemetry.Host = address.String()
	}

	traceURL := &url.URL{
		Host: telemetry.Host,
		Path: httpRouteLabel(ginContext),
	}

	telemetry.URL = traceURL
	telemetry.RequestURI = traceURL.Path
	telemetry.Header = request.Header.Clone()
	telemetry.Header.Del("User-Agent")

	state := &httpTraceState{request: request, telemetry: telemetry}
	ginContext.Set(httpTraceKey{}, state)
	ginContext.Request = telemetry
	defer func() {
		ginContext.Request = state.request
		if state.restoreErrors != nil {
			state.restoreErrors()
		}
	}()

	ginContext.Next()
}

// serveTracedRequest keeps the application request and errors intact while
// restricting the values otelgin sees before and after the handler chain.
func serveTracedRequest(ginContext *gin.Context) {
	value, _ := ginContext.Get(httpTraceKey{})
	state := value.(*httpTraceState)
	baseCtx := state.request.Context()
	ginContext.Request = state.request.WithContext(ginContext.Request.Context())
	ginContext.Next()

	state.request = ginContext.Request.WithContext(baseCtx)
	ginContext.Request = state.telemetry.WithContext(ginContext.Request.Context())
	if len(ginContext.Errors) != 0 {
		originalErrors := ginContext.Errors
		state.restoreErrors = func() { ginContext.Errors = originalErrors }
		ginContext.Errors = nil
		_ = ginContext.Error(errors.New("tserver: HTTP handler reported an error"))
	}
}
