package tserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/choveylee/tlog"
	"github.com/gin-gonic/gin"
)

// defaultShutdownTimeout bounds how long [http.Server.Shutdown] waits for active connections
// to finish after a stop signal or parent context cancellation.
const defaultShutdownTimeout = 30 * time.Second

// SetHttpServerMode configures Gin's global mode: [gin.DebugMode] enables debug output;
// any other value selects release mode via [gin.ReleaseMode].
func SetHttpServerMode(runMode string) {
	if runMode == gin.DebugMode {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
}

// StartHttpServerTLS listens on the given TCP port with HTTPS using certFile and keyFile,
// and runs until ctx is cancelled or SIGINT/SIGTERM is received. It performs a graceful
// shutdown via [http.Server.Shutdown] with an upper time limit of [defaultShutdownTimeout].
func StartHttpServerTLS(ctx context.Context, router *gin.Engine, httpPort int, certFile, keyFile string) {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", httpPort),
		Handler: router,
	}

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	defer signal.Stop(shutdownChan)

	go func() {
		err := server.ListenAndServeTLS(certFile, keyFile)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			tlog.F(ctx).Err(err).Msgf("start http server (%d) err (%v).",
				httpPort, err)
		}
	}()

	tlog.I(ctx).Msgf("http server started, listen on %d.", httpPort)

	select {
	case <-ctx.Done():
		shutdownHTTPServer(ctx, server)
	case <-shutdownChan:
		shutdownHTTPServer(ctx, server)
	}
}

// StartHttpServer listens on the given TCP port over HTTP and runs until ctx is cancelled
// or SIGINT/SIGTERM is received. It performs a graceful shutdown via [http.Server.Shutdown]
// with an upper time limit of [defaultShutdownTimeout].
func StartHttpServer(ctx context.Context, router *gin.Engine, httpPort int) {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", httpPort),
		Handler: router,
	}

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	defer signal.Stop(shutdownChan)

	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			tlog.F(ctx).Err(err).Msgf("start http server (%d) err (%v).",
				httpPort, err)
		}
	}()

	tlog.I(ctx).Msgf("http server started, listen on %d.", httpPort)

	select {
	case <-ctx.Done():
		shutdownHTTPServer(ctx, server)
	case <-shutdownChan:
		shutdownHTTPServer(ctx, server)
	}
}

// shutdownHTTPServer calls [http.Server.Shutdown] with a fresh context limited by
// [defaultShutdownTimeout]. A separate context is required because the caller's ctx may
// already be cancelled when stopping, which would otherwise cause Shutdown to return
// immediately without draining connections.
func shutdownHTTPServer(logCtx context.Context, srv *http.Server) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()

	err := srv.Shutdown(shutdownCtx)
	if err != nil {
		tlog.E(logCtx).Err(err).Msg("shutdown http server err.")
	}
}
