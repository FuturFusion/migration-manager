package worker_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/internal/worker"
)

var lsblk = `
{
   "blockdevices": [
      {
         "name": "sda",
         "fstype": null,
         "children": [
            {
               "name": "sda1",
               "fstype": "ext4"
            },{
               "name": "sda2",
               "fstype": "ext3"
            },{
               "name": "sda3",
               "fstype": "swap"
            },{
               "name": "sda4",
               "fstype": "btrfs"
            }
         ]
      }
   ]
}
`

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

func TestLSBLKUnmarshaling(t *testing.T) {
	lsblkOutput := util.LSBLKOutput{}
	err := json.Unmarshal([]byte(lsblk), &lsblkOutput)
	require.NoError(t, err)
	require.Len(t, lsblkOutput.BlockDevices, 1)
	require.Equal(t, "sda", lsblkOutput.BlockDevices[0].Name)
	require.Equal(t, "sda1", lsblkOutput.BlockDevices[0].Children[0].Name)
	require.Equal(t, "ext4", lsblkOutput.BlockDevices[0].Children[0].FSType)
	require.Equal(t, "sda2", lsblkOutput.BlockDevices[0].Children[1].Name)
	require.Equal(t, "ext3", lsblkOutput.BlockDevices[0].Children[1].FSType)
	require.Equal(t, "sda3", lsblkOutput.BlockDevices[0].Children[2].Name)
	require.Equal(t, "swap", lsblkOutput.BlockDevices[0].Children[2].FSType)
	require.Equal(t, "sda4", lsblkOutput.BlockDevices[0].Children[3].Name)
	require.Equal(t, "btrfs", lsblkOutput.BlockDevices[0].Children[3].FSType)
}

func TestLVSUnmarshaling(t *testing.T) {
	novg := worker.LVSOutput{}
	err := json.Unmarshal([]byte(noVGs), &novg)
	require.NoError(t, err)
	require.Empty(t, novg.Report[0].LV)

	onevg := worker.LVSOutput{}
	err = json.Unmarshal([]byte(oneVG), &onevg)
	require.NoError(t, err)
	require.Len(t, onevg.Report[0].LV, 1)
	require.Equal(t, "ubuntu-vg", onevg.Report[0].LV[0].VGName)
	require.Equal(t, "ubuntu-lv", onevg.Report[0].LV[0].LVName)
}
