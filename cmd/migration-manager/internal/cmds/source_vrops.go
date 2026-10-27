package cmds

import (
	"fmt"

	"github.com/FuturFusion/migration-manager/shared/api"
)

func updateVROpsProperties(asker Asker, props *api.VROpsProperties) error {
	props.SetDefaults()
	var err error
	props.Endpoint, err = asker.AskString("Endpoint [default="+props.Endpoint+"]: ", props.Endpoint, nil)
	if err != nil {
		return err
	}

	updateAuth, err := asker.AskBool("Update configured authentication? (yes/no) [default=no]: ", "no")
	if err != nil {
		return err
	}

	if updateAuth {
		props.Username, err = asker.AskString("Username [default="+props.Username+"]: ", props.Username, nil)
		if err != nil {
			return err
		}

		props.Password, err = asker.AskPasswordOnce("Password: ")
		if err != nil {
			return err
		}
	}

	props.AuthSource, err = asker.AskString("Authentication source [default="+props.AuthSource+"]: ", props.AuthSource, nil)
	if err != nil {
		return err
	}

	for _, field := range []struct {
		label string
		value *api.Duration
	}{
		{"Connection timeout", &props.ConnectionTimeout},
		{"Monitoring sync timeout", &props.SyncTimeout},
	} {
		value, err := asker.AskString(fmt.Sprintf("%s [default=%s]: ", field.label, field.value.String()), field.value.String(), nil)
		if err != nil {
			return err
		}

		duration, err := api.ParseDuration(value)
		if err != nil {
			return err
		}

		if duration.Duration <= 0 {
			return fmt.Errorf("%s must be positive", field.label)
		}
		*field.value = duration
	}

	props.TrustedServerCertificateFingerprint, err = asker.AskString("Trusted TLS certificate fingerprint ["+props.TrustedServerCertificateFingerprint+"]: ", props.TrustedServerCertificateFingerprint, validateSHA256Format)
	return err
}
