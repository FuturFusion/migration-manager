//go:build linux

package nbdcopy_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lxc/incus/v7/shared/subprocess"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migratekit/nbdcopy"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name   string
		script string
		stderr string
		failed bool
	}{
		{name: "success", script: "#!/bin/sh\nexit 0\n"},
		{
			name:   "stderr preserved",
			script: "#!/bin/sh\necho 'VixDiskLib_Open: The server refused connection' >&2\nexit 1\n",
			stderr: "VixDiskLib_Open: The server refused connection\n",
			failed: true,
		},
		{name: "empty stderr", script: "#!/bin/sh\nexit 1\n", failed: true},
		{name: "start failure", failed: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binDir := t.TempDir()
			t.Setenv("PATH", binDir)
			if tt.script != "" {
				err := os.WriteFile(filepath.Join(binDir, "nbdcopy"), []byte(tt.script), 0o755)
				require.NoError(t, err)
			}

			err := nbdcopy.Run("Importing disk", "source", "destination", 1024, false, "disk", func(string, bool) {})
			if !tt.failed {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			require.Contains(t, err.Error(), "Failed to run: nbdcopy --progress=3 source destination")
			var runErr subprocess.RunError
			require.ErrorAs(t, err, &runErr)
			require.Equal(t, tt.stderr, runErr.StdErr().String())
			if tt.script == "" {
				var startErr *exec.Error
				require.ErrorAs(t, err, &startErr)
				return
			}

			require.Contains(t, err.Error(), "exit status 1")
			if tt.stderr != "" {
				require.Contains(t, err.Error(), "VixDiskLib_Open: The server refused connection")
			}

			var exitErr *exec.ExitError
			require.ErrorAs(t, err, &exitErr)
			require.Equal(t, 1, exitErr.ExitCode())
		})
	}
}
