package cmds

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/cmd/migration-manager/internal/config"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestSourceSync(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/1.0/sources/ops/:sync", r.URL.Path)
		_, _ = w.Write([]byte(`{"status_code":200,"status":"Success"}`))
	}))
	defer server.Close()
	syncCmd := cmdSourceSync{global: &CmdGlobal{config: &config.Config{
		DefaultRemote: "default",
		Remotes: map[string]config.Remote{"default": {
			Addr: server.URL, AuthType: config.AuthTypeTLS,
			ServerCert: api.Certificate{Certificate: server.Certificate()},
		}},
	}}}
	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	require.NoError(t, syncCmd.Run(cmd, []string{"ops"}))
	require.Contains(t, output.String(), "Successfully synced")
	require.Error(t, syncCmd.Run(cmd, nil))
}
