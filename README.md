# tserver

**tserver** is a Go library that provides HTTP server construction and lifecycle
helpers for applications built on [Gin](https://github.com/gin-gonic/gin). It
integrates OpenTelemetry HTTP tracing, Prometheus-compatible request latency
metrics, structured access logging through
[tlog](https://github.com/choveylee/tlog), and graceful shutdown in response to
context cancellation or operating-system signals (`SIGINT`, `SIGTERM`).

## Features

- **Router bootstrap** - A preconfigured `*gin.Engine` with panic recovery,
  OpenTelemetry instrumentation, metrics, access logging, request-body reuse
  middleware, and a `/healthz` endpoint.
- **Server lifecycle** - Blocking `StartHttpServer` and `StartHttpServerTLS`
  helpers with graceful shutdown through
  [`http.Server.Shutdown`](https://pkg.go.dev/net/http#Server.Shutdown) and a
  bounded wait (default **30 seconds**), independent of a parent context that
  may already be cancelled.
- **Metrics** - The `http_server_request_latency` histogram with labels
  `http_method`, `http_server_route` (using the Gin route template only; unmatched
  routes use `unknown` to avoid high Prometheus cardinality), and `http_status`.
- **Access logs** - Structured method, route, status, client IP, response size
  and timing through tlog. Requests that panic before writing a response are
  recorded with status 500. A panic after the response headers are committed
  aborts the connection or HTTP/2 stream and marks the trace as an error.

## Requirements

- Go **1.27.4** or later (see `go.mod`).

## Installation

```bash
go get github.com/choveylee/tserver@latest
```

## Usage

The following example constructs a router, registers a route, and runs the HTTP
server until the context is cancelled or a stop signal is received.

```go
package main

import (
	"context"

	"github.com/choveylee/tserver"
	"github.com/gin-gonic/gin"
)

func main() {
	tserver.SetHttpServerMode(gin.ReleaseMode)

	router := tserver.NewRouter("my-service")
	router.GET("/api/v1/hello", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "hello"})
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the server. The call blocks until cancel() is invoked or SIGINT/SIGTERM is received.
	if err := tserver.StartHttpServer(ctx, router, 8080); err != nil {
		panic(err)
	}
}
```

For HTTPS, use `StartHttpServerTLS` with certificate and key file paths instead
of `StartHttpServer`.

If you need server-sent events (SSE), long polling, slow uploads, or large
downloads, pass timeout options to `StartHttpServer` or `StartHttpServerTLS`:

```go
if err := tserver.StartHttpServer(
	ctx,
	router,
	8080,
	tserver.WithReadTimeout(0),
	tserver.WithWriteTimeout(0),
); err != nil {
	panic(err)
}
```

### Subpackage `middleware`

Import path: `github.com/choveylee/tserver/middleware`. The package name is
**`tmiddleware`**. It provides optional Gin middleware such as CORS, rate
limiting, and request-body buffering. Compose these on your own `gin.Engine` if
you do not use `NewRouter`, or add them to a router created by `NewRouter` as
needed.

```go
import (
	tmiddleware "github.com/choveylee/tserver/middleware"
)

r := gin.New()
r.Use(tmiddleware.CorsMiddleware())
```

`LimiterMiddleware` derives its default key from Gin's `ClientIP()` and applies
the limiter's `IPv4Mask` or `IPv6Mask`. For example, an IPv4 `/24` mask groups
`192.0.2.1` and `192.0.2.200` under `192.0.2.0`, so they share one quota.
`NewRouter` trusts the socket peer by default. Behind a reverse proxy, use
`router.SetTrustedProxies` with the proxy's IP addresses or CIDRs before serving.
When mounting the limiter on a custom Gin engine, configure trusted proxies on
that engine. The limiter's forwarded-header options apply to its own IP helpers.

`WithKeyGetter` replaces the default key completely, so custom keys are used
without masking. `WithExcludedKey` receives the final key, including the subnet
mask when the default getter is used. Calling `DefaultKeyGetter` directly
continues to return Gin's raw client IP.

The limiter stops the route before invoking a rejection callback. Its default
status is 500 when the limiter or store is unavailable and 429 when the quota
is exhausted. A custom callback can override the status before writing.
When creating a go-redis client for `tlimiter.NewRedisStore`, set
`ContextTimeoutEnabled: true` so request deadlines reach Redis socket I/O.

`CorsMiddleware` answers browser preflights with 204. Plain `OPTIONS` requests
can reach the application's handlers. `ReuseMiddleware` captures bytes as the
handler reads. Complete bodies of up to 1 MiB can be read again after EOF.
Replay follows the active request if middleware copies it with `WithContext`
or `Clone`. Oversized bodies continue streaming and failed reads retain their
error.

## API overview

| Symbol | Description |
|--------|-------------|
| `SetHttpServerMode` | Sets Gin's global mode (`DebugMode` versus `ReleaseMode`). |
| `NewRouter(serviceName string)` | Returns a configured `*gin.Engine` with observability middleware and `/healthz`. |
| `StartHttpServer` | Serves HTTP on `:{port}` until shutdown, returns startup or shutdown errors, and accepts optional timeout overrides. |
| `StartHttpServerTLS` | Serves HTTPS on `:{port}` with the specified certificate and key files, with the same optional timeout overrides. |

Refer to [pkg.go.dev](https://pkg.go.dev/github.com/choveylee/tserver) for complete documentation.

## Observability

- **Tracing** - `otelgin` receives the service name and route-level request
  metadata. Raw paths, query strings, user agents and handler error text are
  omitted from its built-in trace fields. The request and Gin errors remain
  available to application code. Server address and port come from the service
  name and local listener address, so incoming `Host` values do not add metric
  dimensions.
- **Metrics** - Latency histograms are registered through
  [tmetric](https://github.com/choveylee/tmetric). Ensure your process exposes
  Prometheus scrape targets as required by your deployment.
- **Logging** - Access and recovery logs omit request bodies, raw URL paths,
  query strings, header values and panic values.

## Shutdown behavior

Caller cancellation, SIGINT and SIGTERM start draining. The default deadline
is 30 seconds and can be set with `WithGracefulShutdownTimeout`. Request
contexts inherit caller values and remain active during the drain. At the
deadline the server cancels those contexts, closes remaining HTTP connections
and returns an error matching `context.DeadlineExceeded`.

WebSockets and other hijacked connections are owned by their handlers and need
application-managed shutdown. A handler must observe request cancellation to
finish its own work after a forced close.

## Related modules

This library depends on internal Choveylee components such as **tlog**,
**tmetric**, and optionally **tlimiter** (through the middleware subpackage), in
addition to Gin and OpenTelemetry contrib.
