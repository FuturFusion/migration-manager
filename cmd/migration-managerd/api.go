package main

import(
	"context"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/FuturFusion/migration-manager/internal/server/response"
)

func restServer(d *Daemon) *http.Server {
	mux := mux.NewRouter()
	mux.StrictSlash(false) // Don't redirect to URL with trailing slash.
	mux.SkipClean(true)
	mux.UseEncodedPath() // Allow encoded values in path segments.

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		_ = response.SyncResponse(true, []string{"/1.0"}).Render(w)
	})

	mux.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = response.NotFound(nil).Render(w)
	})

	return &http.Server{
		Handler:     mux,
		ConnContext: SaveConnectionInContext,
		IdleTimeout: 30 * time.Second,
	}
}

// SaveConnectionInContext can be set as the ConnContext field of a http.Server to set the connection
// in the request context for later use.
func SaveConnectionInContext(ctx context.Context, connection net.Conn) context.Context {
	return context.WithValue(ctx, "conn", connection)
}
