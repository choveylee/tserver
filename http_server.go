package tserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/choveylee/tlog"
	"github.com/gin-gonic/gin"
)

// defaultShutdownTimeout defines the maximum duration allowed for graceful shutdown after a
// stop signal or parent-context cancellation.
const defaultShutdownTimeout = 30 * time.Second

const (
	defaultReadHeaderTimeout = 5 * time.Second
	defaultReadTimeout       = 30 * time.Second
	defaultWriteTimeout      = 30 * time.Second
	defaultIdleTimeout       = 120 * time.Second
)

// HttpServerOption customizes the [http.Server] instances created by [StartHttpServer] and
// [StartHttpServerTLS].
type HttpServerOption interface {
	applyOption(*http.Server)
}

type httpServerOption func(*http.Server)

func (option httpServerOption) applyOption(server *http.Server) {
	option(server)
}

// WithReadHeaderTimeout configures [http.Server.ReadHeaderTimeout]. Passing zero or a
// negative duration disables the timeout, consistent with net/http semantics.
func WithReadHeaderTimeout(timeout time.Duration) HttpServerOption {
	return httpServerOption(func(server *http.Server) {
		server.ReadHeaderTimeout = timeout
	})
}

// WithReadTimeout configures [http.Server.ReadTimeout]. Passing zero or a negative
// duration disables the timeout, consistent with net/http semantics.
func WithReadTimeout(timeout time.Duration) HttpServerOption {
	return httpServerOption(func(server *http.Server) {
		server.ReadTimeout = timeout
	})
}

// WithWriteTimeout configures [http.Server.WriteTimeout]. Passing zero or a negative
// duration disables the timeout, consistent with net/http semantics.
func WithWriteTimeout(timeout time.Duration) HttpServerOption {
	return httpServerOption(func(server *http.Server) {
		server.WriteTimeout = timeout
	})
}

// WithIdleTimeout configures [http.Server.IdleTimeout]. Passing zero or a negative
// duration disables the timeout, consistent with net/http semantics.
func WithIdleTimeout(timeout time.Duration) HttpServerOption {
	return httpServerOption(func(server *http.Server) {
		server.IdleTimeout = timeout
	})
}

// SetHttpServerMode sets Gin's global execution mode. [gin.DebugMode] enables debug
// behavior; any other value selects [gin.ReleaseMode].
func SetHttpServerMode(runMode string) {
	if runMode == gin.DebugMode {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
}

// StartHttpServerTLS serves HTTPS on the specified TCP port using certFile and keyFile
// until ctx is cancelled or SIGINT/SIGTERM is received. It returns any startup or serve
// error to the caller after logging it, and performs graceful shutdown through
// [http.Server.Shutdown] with an upper time limit of [defaultShutdownTimeout].
//
// Optional [HttpServerOption] values may override the default [http.Server] timeouts for
// use cases such as server-sent events (SSE), long polling, or large transfers.
func StartHttpServerTLS(ctx context.Context, router *gin.Engine, httpPort int, certFile, keyFile string, options ...HttpServerOption) error {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", httpPort),
		Handler: router,

		ReadHeaderTimeout: defaultReadHeaderTimeout,
		ReadTimeout:       defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
	}

	for _, option := range options {
		if option != nil {
			option.applyOption(server)
		}
	}

	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("load tls certificate for port %d: %w", httpPort, err)
	}

	server.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{certificate},
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", server.Addr, err)
	}

	return serveHTTPServer(ctx, server, listener, httpPort, func() error {
		return server.ServeTLS(listener, "", "")
	})
}

// StartHttpServer serves HTTP on the specified TCP port until ctx is cancelled or
// SIGINT/SIGTERM is received. It returns any startup or serve error to the caller after
// logging it, and performs graceful shutdown through [http.Server.Shutdown] with an upper
// time limit of [defaultShutdownTimeout].
//
// Optional [HttpServerOption] values may override the default [http.Server] timeouts for
// use cases such as server-sent events (SSE), long polling, or large transfers.
func StartHttpServer(ctx context.Context, router *gin.Engine, httpPort int, options ...HttpServerOption) error {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", httpPort),
		Handler: router,

		ReadHeaderTimeout: defaultReadHeaderTimeout,
		ReadTimeout:       defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
	}

	for _, option := range options {
		if option != nil {
			option.applyOption(server)
		}
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", server.Addr, err)
	}

	return serveHTTPServer(ctx, server, listener, httpPort, func() error {
		return server.Serve(listener)
	})
}

// shutdownHTTPServer calls [http.Server.Shutdown] with a fresh context limited by
// [defaultShutdownTimeout]. A separate context is required because the caller's context
// may already be cancelled when shutdown begins, which would otherwise cause Shutdown to
// return immediately without draining active connections.
func shutdownHTTPServer(logCtx context.Context, srv *http.Server) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()

	err := srv.Shutdown(shutdownCtx)
	if err != nil {
		tlog.E(logCtx).Err(err).Msg("HTTP server shutdown failed")
	}

	return err
}

func serveHTTPServer(ctx context.Context, server *http.Server, listener net.Listener, httpPort int, serve func() error) error {
	defer func() {
		_ = listener.Close()
	}()

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(shutdownChan)

	serveErrChan := make(chan error, 1)
	go func() {
		err := serve()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErrChan <- err

			return
		}

		serveErrChan <- nil
	}()

	tlog.I(ctx).Msgf("HTTP server started on port %d", httpPort)

	select {
	case err := <-serveErrChan:
		if err != nil {
			tlog.F(ctx).Err(err).Msgf("HTTP server returned an unexpected error on port %d", httpPort)
		}

		return err
	case <-ctx.Done():
	case <-shutdownChan:
	}

	shutdownErr := shutdownHTTPServer(ctx, server)
	serveErr := <-serveErrChan

	if shutdownErr != nil && serveErr != nil {
		return errors.Join(shutdownErr, serveErr)
	}

	if shutdownErr != nil {
		return shutdownErr
	}

	return serveErr
}
