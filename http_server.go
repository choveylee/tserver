package tserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
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

// HttpServerOption customizes servers created by [StartHttpServer] and
// [StartHttpServerTLS].
type HttpServerOption interface {
	applyOption(*httpServerConfig)
}

type httpServerConfig struct {
	server          *http.Server
	shutdownTimeout time.Duration
}

type httpServerOption func(*httpServerConfig)

func (option httpServerOption) applyOption(config *httpServerConfig) {
	option(config)
}

// WithReadHeaderTimeout configures [http.Server.ReadHeaderTimeout]. Passing zero or a
// negative duration disables the header deadline independently of ReadTimeout.
func WithReadHeaderTimeout(timeout time.Duration) HttpServerOption {
	return httpServerOption(func(config *httpServerConfig) {
		config.server.ReadHeaderTimeout = disabledFallbackTimeout(timeout)
	})
}

// WithReadTimeout configures [http.Server.ReadTimeout]. Passing zero or a negative
// duration disables the timeout.
func WithReadTimeout(timeout time.Duration) HttpServerOption {
	return httpServerOption(func(config *httpServerConfig) {
		config.server.ReadTimeout = timeout
	})
}

// WithWriteTimeout configures [http.Server.WriteTimeout]. Passing zero or a negative
// duration disables the timeout.
func WithWriteTimeout(timeout time.Duration) HttpServerOption {
	return httpServerOption(func(config *httpServerConfig) {
		config.server.WriteTimeout = timeout
	})
}

// WithIdleTimeout configures [http.Server.IdleTimeout]. Passing zero or a negative
// duration disables the idle deadline independently of ReadTimeout.
func WithIdleTimeout(timeout time.Duration) HttpServerOption {
	return httpServerOption(func(config *httpServerConfig) {
		config.server.IdleTimeout = disabledFallbackTimeout(timeout)
	})
}

// disabledFallbackTimeout prevents net/http from falling back to ReadTimeout.
func disabledFallbackTimeout(timeout time.Duration) time.Duration {
	if timeout == 0 {
		return -1
	}

	return timeout
}

// WithGracefulShutdownTimeout sets the drain deadline. A nonpositive timeout
// uses the default of 30 seconds.
func WithGracefulShutdownTimeout(timeout time.Duration) HttpServerOption {
	return httpServerOption(func(config *httpServerConfig) {
		config.shutdownTimeout = defaultShutdownTimeout
		if timeout > 0 {
			config.shutdownTimeout = timeout
		}
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

// StartHttpServerTLS serves HTTPS until cancellation or a termination signal.
// It loads the certificate before binding and returns startup or shutdown errors.
func StartHttpServerTLS(ctx context.Context, router *gin.Engine, httpPort int, certFile, keyFile string, options ...HttpServerOption) error {
	config, err := newHTTPServerConfig(ctx, router, httpPort, options)
	if err != nil {
		return err
	}

	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("tserver: load TLS certificate for port %d: %w", httpPort, err)
	}

	tlsConfig := &tls.Config{Certificates: []tls.Certificate{certificate}}

	config.server.TLSConfig = tlsConfig

	return serveHTTPServer(ctx, config)
}

// StartHttpServer serves HTTP until cancellation or a termination signal and
// drains active requests before returning.
func StartHttpServer(ctx context.Context, router *gin.Engine, httpPort int, options ...HttpServerOption) error {
	config, err := newHTTPServerConfig(ctx, router, httpPort, options)
	if err != nil {
		return err
	}

	return serveHTTPServer(ctx, config)
}

// newHTTPServerConfig validates the input shared by HTTP and TLS startup.
func newHTTPServerConfig(ctx context.Context, router *gin.Engine, httpPort int, options []HttpServerOption) (*httpServerConfig, error) {
	if ctx == nil {
		return nil, errors.New("tserver: server context must not be nil")
	}

	if router == nil {
		return nil, errors.New("tserver: router must not be nil")
	}

	if httpPort < 0 || httpPort > 65535 {
		return nil, fmt.Errorf("tserver: HTTP port must be between 0 and 65535, got %d", httpPort)
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("tserver: start HTTP server: %w", err)
	}

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", httpPort),
		Handler:           router,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		ReadTimeout:       defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
	}
	config := &httpServerConfig{server: server, shutdownTimeout: defaultShutdownTimeout}
	for _, option := range options {
		if option != nil {
			option.applyOption(config)
		}
	}

	return config, nil
}

// shutdownHTTPServer drains with a fresh deadline and cancels remaining request
// contexts before force-closing connections when the server cannot drain.
func shutdownHTTPServer(ctx context.Context, server *http.Server, cancelRequests context.CancelFunc, timeout time.Duration) error {
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		cancelRequests()
		closeErr := server.Close()

		return fmt.Errorf("tserver: drain HTTP server: %w", errors.Join(err, closeErr))
	}

	return nil
}

// serveHTTPServer binds the listener and coordinates serving, signals and shutdown.
func serveHTTPServer(ctx context.Context, config *httpServerConfig) error {
	stopCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server := config.server
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(stopCtx, "tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("tserver: listen on %s: %w", server.Addr, err)
	}

	defer listener.Close()

	if err := stopCtx.Err(); err != nil {
		return fmt.Errorf("tserver: start HTTP server: %w", err)
	}

	// Parent cancellation starts draining. Active requests keep their context
	// values and are canceled only after the drain or when it times out.
	requestCtx, cancelRequests := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelRequests()
	server.BaseContext = func(net.Listener) context.Context { return requestCtx }

	serveResult := make(chan error, 1)
	go func() {
		var err error
		if server.TLSConfig != nil {
			err = server.ServeTLS(listener, "", "")
		} else {
			err = server.Serve(listener)
		}

		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		} else if err != nil {
			err = fmt.Errorf("tserver: serve HTTP on %s: %w", listener.Addr(), err)
		}

		serveResult <- err
	}()

	tlog.I(ctx).Msgf("Start HTTP listener (address=%s)", listener.Addr())

	select {
	case err := <-serveResult:
		if err != nil {
			cancelRequests()

			return errors.Join(err, server.Close())
		}

		return nil
	case <-stopCtx.Done():
	}

	shutdownErr := shutdownHTTPServer(ctx, server, cancelRequests, config.shutdownTimeout)

	return errors.Join(shutdownErr, <-serveResult)
}
