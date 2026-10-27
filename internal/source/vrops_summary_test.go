package source

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestVROpsMonitoringSummary(t *testing.T) {
	for _, tc := range []struct {
		period   string
		duration time.Duration
	}{
		{"", 10 * time.Minute},
		{"10m", 10 * time.Minute},
		{"1h", time.Hour},
		{"1d", 24 * time.Hour},
		{"1w", 7 * 24 * time.Hour},
	} {
		t.Run(tc.period, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				assert.Equal(t, "/suite-api/api/resources/resource/stats", r.URL.Path)
				query := r.URL.Query()
				assert.ElementsMatch(t, []string{
					"cpu|usage_average", "mem|usage_average", "guestfilesystem|usage_total", "guestfilesystem|capacity_total",
					"storage|usage_average", "storage|commandsAveraged_average", "net|transmitted_average",
				}, query["statKey"])
				assert.Equal(t, "false", query.Get("currentOnly"))
				begin, err := strconv.ParseInt(query.Get("begin"), 10, 64)
				assert.NoError(t, err)
				end, err := strconv.ParseInt(query.Get("end"), 10, 64)
				assert.NoError(t, err)
				assert.Equal(t, tc.duration.Milliseconds(), end-begin)
				stat := func(key string, data []any, times []int64) map[string]any {
					return map[string]any{"statKey": map[string]string{"key": key}, "data": data, "timestamps": times}
				}

				err = json.NewEncoder(w).Encode(map[string]any{"values": []any{
					map[string]any{"resourceId": "resource", "stat-list": map[string]any{"stat": []any{
						stat("cpu|usage_average", []any{60, 20, 20, nil, -1, 99, 99}, []int64{end, begin, begin, begin + 1, begin + 2, begin - 1, end + 1}),
						stat("mem|usage_average", []any{30}, []int64{}),
						stat("guestfilesystem|usage_total", []any{180, 120}, []int64{end, begin}),
						stat("guestfilesystem|capacity_total", []any{512, 256}, []int64{end, begin}),
						stat("storage|usage_average", []any{1024, 3072}, []int64{begin, end}),
						stat("storage|commandsAveraged_average", []any{10, 30}, []int64{begin, end}),
						stat("net|transmitted_average", []any{0, 0}, []int64{begin, end}),
						stat("unrequested", []any{99}, []int64{end}),
					}}},
					map[string]any{"resourceId": "other", "stat-list": map[string]any{"stat": []any{stat("cpu|usage_average", []any{99}, []int64{end})}}},
				}})
				assert.NoError(t, err)
			}))
			defer server.Close()
			client := InternalVROpsSource{
				InternalSource: InternalSource{isConnected: true}, VROpsProperties: api.VROpsProperties{Endpoint: server.URL},
				client: server.Client(), token: "token",
			}

			summary, err := client.GetMonitoringSummary(t.Context(), "resource", tc.period)
			require.NoError(t, err)
			require.Equal(t, tc.duration, summary.End.Sub(summary.Begin))
			require.Equal(t, map[string]float64{
				"cpu|usage_average": 40, "guestfilesystem|usage_total": 180, "guestfilesystem|capacity_total": 512,
				"storage|usage_average": 2048, "storage|commandsAveraged_average": 20, "net|transmitted_average": 0,
			}, summary.Metrics)
			_, err = client.GetMonitoringSummary(t.Context(), "resource", "1mo")
			require.ErrorContains(t, err, "Invalid monitoring period")
			require.Equal(t, 1, requests)
		})
	}
}
