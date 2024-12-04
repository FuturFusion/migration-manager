package main

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/lxc/incus/v6/shared/logger"

	"github.com/FuturFusion/migration-manager/internal/server/response"
)

// swagger:operation GET / server api_get
//
//	Get the supported API endpoints
//
//	Returns a list of supported API versions (URLs).
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API endpoints
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          type: string
//	          description: Response type
//	          example: sync
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          type: array
//	          description: List of endpoints
//	          items:
//	            type: string
//	          example: ["/1.0"]
func restServer(d *Daemon) *http.Server {
	router := mux.NewRouter()
	router.StrictSlash(false) // Don't redirect to URL with trailing slash.
	router.SkipClean(true)
	router.UseEncodedPath() // Allow encoded values in path segments.

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		_ = response.SyncResponse(true, []string{"/1.0"}).Render(w)
	})

	for _, c := range api10 {
		d.createCmd(router, "1.0", c)
	}

	router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Info("Sending top level 404", logger.Ctx{"url": r.URL, "method": r.Method, "remote": r.RemoteAddr})
		w.Header().Set("Content-Type", "application/json")
		_ = response.NotFound(nil).Render(w)
	})

	return &http.Server{
		Handler:     router,
		ConnContext: SaveConnectionInContext,
		IdleTimeout: 30 * time.Second,
	}
}

type contextKey string

// SaveConnectionInContext can be set as the ConnContext field of a http.Server to set the connection
// in the request context for later use.
func SaveConnectionInContext(ctx context.Context, connection net.Conn) context.Context {
	return context.WithValue(ctx, contextKey("conn"), connection)
}
