package web

import (
	"context"
	"net/http"
	"time"

	"backend/pkg/log"
)

func Start(server *http.Server) {
	log.Infow("starting an http server", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		// cannot panic, because this probably is an intentional close
		log.Infow("shutting down the http server", "message", err.Error())
	}
}

func Shutdown(server *http.Server, shutdownTimeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Errorw("failed to shutdown the http server", "error", err.Error())
		return
	}
	log.Info("http server stopped")
}
