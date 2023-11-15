/**
 * @Author: lidonglin
 * @Description:
 * @File:  http_server.go
 * @Version: 1.0.0
 * @Date: 2023/11/15 11:46
 */

package tserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/choveylee/tlog"
	"github.com/gin-gonic/gin"
)

func SetHttpServerMode(runMode string) {
	if runMode == gin.DebugMode {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
}

func StartHttpServerTLS(ctx context.Context, router *gin.Engine, httpPort int, certFile, keyFile string) {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", httpPort),
		Handler: router,
	}

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

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
		err := server.Shutdown(ctx)
		if err != nil {
			tlog.E(ctx).Err(err).Msgf("shutdown http server err (%v).",
				err)

			return
		}

		return
	case <-shutdownChan:
		err := server.Shutdown(ctx)
		if err != nil {
			tlog.E(ctx).Err(err).Msgf("shutdown http server err (%v).",
				err)

			return
		}
		return
	}
}

func StartHttpServer(ctx context.Context, router *gin.Engine, httpPort int) {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", httpPort),
		Handler: router,
	}

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

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
		err := server.Shutdown(ctx)
		if err != nil {
			tlog.E(ctx).Err(err).Msgf("shutdown http server err (%v).",
				err)

			return
		}

		return
	case <-shutdownChan:
		err := server.Shutdown(ctx)
		if err != nil {
			tlog.E(ctx).Err(err).Msgf("shutdown http server err (%v).",
				err)

			return
		}
		return
	}
}
