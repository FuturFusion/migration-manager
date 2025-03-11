package api

import (
	"crypto/tls"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/FuturFusion/migration-manager/internal/server/request"
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
	router := http.NewServeMux()

	uiDir := uiHTTPDir{http.Dir(filepath.Join(d.os.VarDir, "ui"))}
	fileServer := http.FileServer(uiDir)

	router.Handle("/ui/", http.StripPrefix("/ui/", fileServer))
	router.HandleFunc("/ui", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui/", http.StatusMovedPermanently)
	})

	// OIDC browser login
	router.HandleFunc("/oidc/login", func(w http.ResponseWriter, r *http.Request) {
		if d.oidcVerifier == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		d.oidcVerifier.Login(w, r)
	})

	router.HandleFunc("/oidc/callback", func(w http.ResponseWriter, r *http.Request) {
		if d.oidcVerifier == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		d.oidcVerifier.Callback(w, r)
	})

	router.HandleFunc("/oidc/logout", func(w http.ResponseWriter, r *http.Request) {
		if d.oidcVerifier == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		d.oidcVerifier.Logout(w, r)
	})

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path != "/" {
			slog.Info("Sending top level 404", slog.Any("url", r.URL), slog.String("method", r.Method), slog.String("remote", r.RemoteAddr))
			_ = response.NotFound(nil).Render(w)
			return
		}

		ua := r.Header.Get("User-Agent")
		if strings.Contains(ua, "Gecko") {
			// Web browser handling.
			http.Redirect(w, r, "/ui/", http.StatusMovedPermanently)
		}

		_ = response.SyncResponse(true, []string{"/1.0"}).Render(w)
	})

	for _, c := range api10 {
		d.createCmd(router, "1.0", c)
	}

	return &http.Server{
		Handler:     router,
		ConnContext: request.SaveConnectionInContext,
		IdleTimeout: 30 * time.Second,
		TLSConfig: &tls.Config{
			NextProtos: []string{"h2", "http/1.1"},
			ClientAuth: tls.RequestClientCert,
		},
	}
}

type uiHTTPDir struct {
	http.FileSystem
}

// Open is part of the http.FileSystem interface.
func (httpFS uiHTTPDir) Open(name string) (http.File, error) {
	fsFile, err := httpFS.FileSystem.Open(name)
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		return httpFS.FileSystem.Open("index.html")
	}

	return fsFile, err
}
