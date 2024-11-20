package worker_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/worker"
)

var noVGs = `
  {
      "report": [
          {
              "lv": [
              ]
          }
      ]
  }
`

var oneVG = `
  {
      "report": [
          {
              "lv": [
                  {"vg_name":"ubuntu-vg", "lv_name":"ubuntu-lv"}
              ]
          }
      ]
  }
`

func TestLVSUnmarshaling(t *testing.T) {
	novg := worker.LVSOutput{}
	err := json.Unmarshal([]byte(noVGs), &novg)
	require.NoError(t, err)
	require.Equal(t, len(novg.Report[0].LV), 0)

	onevg := worker.LVSOutput{}
	err = json.Unmarshal([]byte(oneVG), &onevg)
	require.NoError(t, err)
	require.Equal(t, len(onevg.Report[0].LV), 1)
	require.Equal(t, onevg.Report[0].LV[0].VGName, "ubuntu-vg")
	require.Equal(t, onevg.Report[0].LV[0].LVName, "ubuntu-lv")
}
