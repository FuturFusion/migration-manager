package worker_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

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

// REVIEW: not sure about these tests, since they are mainly testing the json
// package and not our own code.

func TestLSBLKUnmarshaling(t *testing.T) {
	lsblkOutput := worker.LSBLKOutput{}
	err := json.Unmarshal([]byte(lsblk), &lsblkOutput)
	require.NoError(t, err)
	require.Equal(t, len(lsblkOutput.BlockDevices), 1)
	require.Equal(t, lsblkOutput.BlockDevices[0].Name, "sda")
	require.Equal(t, lsblkOutput.BlockDevices[0].Children[0].Name, "sda1")
	require.Equal(t, lsblkOutput.BlockDevices[0].Children[0].FSType, "ext4")
	require.Equal(t, lsblkOutput.BlockDevices[0].Children[1].Name, "sda2")
	require.Equal(t, lsblkOutput.BlockDevices[0].Children[1].FSType, "ext3")
	require.Equal(t, lsblkOutput.BlockDevices[0].Children[2].Name, "sda3")
	require.Equal(t, lsblkOutput.BlockDevices[0].Children[2].FSType, "swap")
	require.Equal(t, lsblkOutput.BlockDevices[0].Children[3].Name, "sda4")
	require.Equal(t, lsblkOutput.BlockDevices[0].Children[3].FSType, "btrfs")
}

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
