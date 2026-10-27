package cmds

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestVROpsPropertiesUpdate(t *testing.T) {
	props := api.VROpsProperties{Endpoint: "https://ops", Username: "admin", Password: "secret", TrustedServerCACertificates: []string{"existing CA"}}
	asker := &AskerMock{
		AskBoolFunc: func(string, string) (bool, error) { return false, nil },
		AskStringFunc: func(question string, defaultValue string, validator func(string) error) (string, error) {
			switch {
			case strings.HasPrefix(question, "Authentication source"):
				return "Directory", nil
			case strings.HasPrefix(question, "Monitoring sync timeout"):
				return "2m", nil
			default:
				return defaultValue, nil
			}
		},
	}

	require.NoError(t, updateVROpsProperties(asker, &props))
	require.Equal(t, "Directory", props.AuthSource)
	require.Equal(t, api.AsDuration(2*time.Minute), props.SyncTimeout)
	require.Equal(t, "secret", props.Password)
	require.Equal(t, []string{"existing CA"}, props.TrustedServerCACertificates)
	require.Empty(t, asker.AskIntCalls())
	require.Empty(t, asker.AskPasswordOnceCalls())
}
