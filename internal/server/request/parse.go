package request

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/lxc/incus/v6/shared/api"
)

// ProjectParam returns the project query parameter from the given request or "default" if parameter is not set.
func ProjectParam(request *http.Request) string {
	projectParam := QueryParam(request, "project")
	if projectParam == "" {
		projectParam = api.ProjectDefaultName
	}

	return projectParam
}

// QueryParam extracts the given query parameter directly from the URL, never from an
// encoded body.
func QueryParam(request *http.Request, key string) string {
	var values url.Values
	var err error

	if request.URL != nil {
		values, err = url.ParseQuery(request.URL.RawQuery)
		if err != nil {
			slog.Warn("Failed to parse query string", slog.String("query", request.URL.RawQuery), slog.Any("error", err))
			return ""
		}
	}

	if values == nil {
		values = make(url.Values)
	}

	return values.Get(key)
}
