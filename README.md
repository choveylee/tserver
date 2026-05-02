# tserver

**tserver** is a Go library that provides HTTP server construction and lifecycle helpers for applications built on [Gin](https://github.com/gin-gonic/gin). It integrates OpenTelemetry HTTP tracing, Prometheus-style request latency metrics, structured access logging (via [tlog](https://github.com/choveylee/tlog)), and graceful shutdown in response to context cancellation or operating-system signals (`SIGINT`, `SIGTERM`).

## Features

- **Router bootstrap** — Preconfigured `*gin.Engine` with panic recovery, OpenTelemetry instrumentation, metrics, access logging, and request-body reuse middleware, plus a `/healthz` endpoint.
- **Server lifecycle** — Blocking `StartHttpServer` / `StartHttpServerTLS` helpers with graceful shutdown using [`http.Server.Shutdown`](https://pkg.go.dev/net/http#Server.Shutdown) and a bounded wait (default **30 seconds**), independent of a possibly already-cancelled parent context.
- **Metrics** — Histogram `http_server_request_latency` with labels `http_method`, `http_server_route` (Gin route template only; unregistered routes use `unknown` to avoid high Prometheus cardinality), and `http_status`.
- **Access logs** — Structured fields via tlog; for failed mutating requests (`POST`, `PUT`, `PATCH`, `DELETE`) with status ≥ 400, request body is logged up to **1 KiB**, with a truncation marker when applicable.

## Requirements

- Go **1.25** or later (see `go.mod`).

## Installation

```bash
go get github.com/choveylee/tserver@latest
```

## Usage

The following example constructs a router, registers a route, and runs the HTTP server until the context is cancelled or a stop signal is received.

```go
package main

import (
	"context"
	"time"

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

	// Run server (blocks). Stop via cancel(ctx) or SIGINT/SIGTERM.
	if err := tserver.StartHttpServer(ctx, router, 8080); err != nil {
		panic(err)
	}
}
```

For TLS, use `StartHttpServerTLS` with certificate and key file paths instead of `StartHttpServer`.

If you need SSE, long polling, slow uploads, or large downloads, pass timeout options to `StartHttpServer` or `StartHttpServerTLS`:

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

Import path: `github.com/choveylee/tserver/middleware`. The package name is **`tmiddleware`**. It provides optional Gin middleware (for example, CORS, rate limiting, and request-body buffering). Compose these on your own `gin.Engine` if you do not use `NewRouter`, or add routes after `NewRouter` as needed.

```go
import (
	tmiddleware "github.com/choveylee/tserver/middleware"
)

r := gin.New()
r.Use(tmiddleware.CorsMiddleware())
```

## API overview

| Symbol | Description |
|--------|-------------|
| `SetHttpServerMode` | Sets Gin’s global mode (`DebugMode` vs `ReleaseMode`). |
| `NewRouter(serviceName string)` | Returns a configured `*gin.Engine` with observability middleware and `/healthz`. |
| `StartHttpServer` | Serves HTTP on `:{port}` until stop, returns startup/shutdown errors, and accepts optional timeout overrides. |
| `StartHttpServerTLS` | Same as above over HTTPS with the given cert and key files, with the same optional timeout overrides. |

Refer to [pkg.go.dev](https://pkg.go.dev/github.com/choveylee/tserver) for complete documentation.

## Observability

- **Tracing** — `serviceName` passed to `NewRouter` is supplied to OpenTelemetry Gin middleware (`otelgin`).
- **Metrics** — Latency histograms are registered through [tmetric](https://github.com/choveylee/tmetric); ensure your process exposes Prometheus scrape targets as required by your deployment.
- **Logging** — Access logs use tlog; Gin’s default writer output is discarded in `NewRouter` because formatting is delegated to `logFormatter`.

## Shutdown behavior

Shutdown is triggered when the caller’s context is cancelled **or** when `SIGINT` / `SIGTERM` is received. The library invokes `http.Server.Shutdown` with a **fresh** context limited to **30 seconds**, so in-flight requests can drain without relying on an already-cancelled parent context.

## Related modules

This library depends on internal Choveylee components such as **tlog**, **tmetric**, and optionally **tlimiter** (via the middleware subpackage), in addition to Gin and OpenTelemetry contrib.
