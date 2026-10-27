package source

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestVROpsMonitoringHistory(t *testing.T) {
	for _, container := range []string{"stat-list", "stats"} {
		t.Run(container, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/suite-api/api/resources/resource/stats", r.URL.Path)
				assert.Equal(t, "vRealizeOpsToken token", r.Header.Get("Authorization"))
				query := r.URL.Query()
				assert.NotContains(t, query, "resourceId")
				assert.ElementsMatch(t, []string{"cpu|usage_average", "mem|usage_average", "disk|read_average", "disk|write_average"}, query["statKey"])
				assert.NotContains(t, query, "rollUpType")
				assert.NotContains(t, query, "intervalType")
				assert.NotContains(t, query, "latestMaxSamples")
				assert.Equal(t, "false", query.Get("currentOnly"))
				begin, err := strconv.ParseInt(query.Get("begin"), 10, 64)
				assert.NoError(t, err)
				end, err := strconv.ParseInt(query.Get("end"), 10, 64)
				assert.NoError(t, err)
				assert.Equal(t, int64((30*time.Minute)/time.Millisecond), end-begin)
				stat := func(key string, data []any, timestamps []int64) map[string]any {
					return map[string]any{"statKey": map[string]string{"key": key}, "data": data, "timestamps": timestamps}
				}

				err = json.NewEncoder(w).Encode(map[string]any{"values": []any{
					map[string]any{"resourceId": "other", container: map[string]any{"stat": []any{stat("cpu|usage_average", []any{99}, []int64{end})}}},
					map[string]any{"resourceId": "resource", container: map[string]any{"stat": []any{
						stat("cpu|usage_average", []any{20, 0, 99, 99, nil, -1}, []int64{end, begin, begin - 1, end + 1, begin + 1, begin + 2}),
						stat("mem|usage_average", []any{30}, []int64{}),
						stat("disk|read_average", []any{1024}, []int64{end}),
						stat("unrequested", []any{99}, []int64{end}),
					}}},
				}})
				assert.NoError(t, err)
			}))
			defer server.Close()
			client := InternalVROpsSource{
				InternalSource:  InternalSource{isConnected: true},
				VROpsProperties: api.VROpsProperties{Endpoint: server.URL},
				client:          server.Client(), token: "token",
			}

			history, err := client.GetMonitoringHistory(t.Context(), "resource")
			require.NoError(t, err)
			require.Len(t, history.Metrics, 2)
			require.Len(t, history.Metrics["cpu|usage_average"], 2)
			require.Zero(t, history.Metrics["cpu|usage_average"][0].Value)
			require.InDelta(t, 20.0, history.Metrics["cpu|usage_average"][1].Value, 0.001)
			require.InDelta(t, 1024.0, history.Metrics["disk|read_average"][0].Value, 0.001)
		})
	}
}

func TestVROpsRequestError(t *testing.T) {
	client := InternalVROpsSource{VROpsProperties: api.VROpsProperties{Username: "operator", Password: "operator-password"}, token: "private-token"}
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{name: "structured error", body: `{"message":"Invalid query for operator-password using private-token\n(operator)"}`, want: "Invalid query for [redacted] using [redacted] ([redacted])"},
		{name: "empty response", body: ""},
		{name: "proxy response", body: "<html>Bad request</html>"},
		{name: "invalid message", body: `{"message":{"secret":"operator-password"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := &http.Response{Status: "400 Bad Request", Body: io.NopCloser(strings.NewReader(tc.body))}
			defer response.Body.Close()
			err := client.requestError(response)
			require.ErrorContains(t, err, "vROps request failed with status 400 Bad Request")
			if tc.want != "" {
				require.ErrorContains(t, err, tc.want)
			}

			require.NotContains(t, err.Error(), "operator")
			require.NotContains(t, err.Error(), "private-token")
		})
	}
}

func TestVROpsRequestErrorRedaction(t *testing.T) {
	client := InternalVROpsSource{VROpsProperties: api.VROpsProperties{Username: "a", Password: "private-password"}, token: "private-token"}
	response := &http.Response{Status: "400 Bad Request", Body: io.NopCloser(strings.NewReader(`{"message":"a private-password private-token"}`))}
	defer response.Body.Close()
	require.EqualError(t, client.requestError(response), "vROps request failed with status 400 Bad Request: [redacted] [redacted] [redacted]")
}
