package cmds

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	localtls "github.com/lxc/incus/v6/shared/tls"
	"github.com/lxc/incus/v6/shared/util"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	internalUtil "github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

const dateLayout = "2006/01/02 15:04 MST"

type cmdConfigTrust struct {
	global *CmdGlobal
	config *CmdConfig
}

func (c *cmdConfigTrust) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "trust"
	cmd.Short = "Manage trusted clients"
	cmd.Long = `Description:
  Manage trusted clients
`

	// Add certificate
	configTrustAddCertificateCmd := cmdConfigTrustAddCertificate{global: c.global, config: c.config}
	cmd.AddCommand(configTrustAddCertificateCmd.Command())

	// List
	configTrustListCmd := cmdConfigTrustList{global: c.global, config: c.config}
	cmd.AddCommand(configTrustListCmd.Command())

	// Remove
	configTrustRemoveCmd := cmdConfigTrustRemove{global: c.global, config: c.config}
	cmd.AddCommand(configTrustRemoveCmd.Command())

	// Show
	configTrustShowCmd := cmdConfigTrustShow{global: c.global, config: c.config}
	cmd.AddCommand(configTrustShowCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }
	return cmd
}

// Add certificate.
type cmdConfigTrustAddCertificate struct {
	global *CmdGlobal
	config *CmdConfig

	flagName string
}

func (c *cmdConfigTrustAddCertificate) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "add-certificate <cert>"
	cmd.Short = "Add new trusted client certificate"
	cmd.Long = `Description:
  Add new trusted client certificate
`

	cmd.Flags().StringVar(&c.flagName, "name", "", "Alternative certificate name")

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigTrustAddCertificate) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	path := args[0]

	if path == "-" {
		path = "/dev/stdin"
	}

	// Check that the path exists.
	if !util.PathExists(path) {
		return fmt.Errorf("Provided certificate path doesn't exist: %s", path)
	}

	// Load the certificate.
	x509Cert, err := localtls.ReadCert(path)
	if err != nil {
		return err
	}

	var name string
	if c.flagName != "" {
		name = c.flagName
	} else {
		name = filepath.Base(path)
	}

	// Add trust relationship.
	cert := api.Certificate{}
	cert.Certificate = base64.StdEncoding.EncodeToString(x509Cert.Raw)
	cert.Name = name
	cert.Type = api.CertificateTypeClient

	content, err := json.Marshal(cert)
	if err != nil {
		return err
	}

	_, err = c.global.doHTTPRequestV1("/certificates", http.MethodPost, "", content)

	return err
}

// List.
type cmdConfigTrustList struct {
	global *CmdGlobal
	config *CmdConfig

	flagFormat  string
	flagColumns string
}

type certificateColumn struct {
	Name string
	Data func(rowData rowData) string
}

type rowData struct {
	Cert    api.Certificate
	TLSCert *x509.Certificate
}

func (c *cmdConfigTrustList) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "list"
	cmd.Short = "List trusted clients"
	cmd.Long = `Description:
  List trusted clients

The -c option takes a (optionally comma-separated) list of arguments
that control which certificate attributes to output when displaying in table
or csv format.

Default column layout is: ndfe

Column shorthand chars:

	n - Name
	c - Common Name
	f - Fingerprint
	d - Description
	i - Issue date
	e - Expiry date
`

	cmd.Flags().StringVarP(&c.flagColumns, "columns", "c", "ndfe", "Columns")
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "table", "Format (csv|json|table|yaml|compact)")

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigTrustList) parseColumns() ([]certificateColumn, error) {
	columnsShorthandMap := map[rune]certificateColumn{
		'n': {"NAME", c.nameColumnData},
		'c': {"COMMON NAME", c.commonNameColumnData},
		'f': {"FINGERPRINT", c.fingerprintColumnData},
		'd': {"DESCRIPTION", c.descriptionColumnData},
		'i': {"ISSUE DATE", c.issueDateColumnData},
		'e': {"EXPIRY DATE", c.expiryDateColumnData},
	}

	columnList := strings.Split(c.flagColumns, ",")

	columns := []certificateColumn{}
	for _, columnEntry := range columnList {
		if columnEntry == "" {
			return nil, fmt.Errorf("Empty column entry (redundant, leading or trailing command) in '%s'", c.flagColumns)
		}

		for _, columnRune := range columnEntry {
			column, ok := columnsShorthandMap[columnRune]
			if !ok {
				return nil, fmt.Errorf("Unknown column shorthand char '%c' in '%s'", columnRune, columnEntry)
			}

			columns = append(columns, column)
		}
	}

	return columns, nil
}

func (c *cmdConfigTrustList) nameColumnData(rowData rowData) string {
	return rowData.Cert.Name
}

func (c *cmdConfigTrustList) commonNameColumnData(rowData rowData) string {
	return rowData.TLSCert.Subject.CommonName
}

func (c *cmdConfigTrustList) fingerprintColumnData(rowData rowData) string {
	return rowData.Cert.Fingerprint[0:12]
}

func (c *cmdConfigTrustList) descriptionColumnData(rowData rowData) string {
	return rowData.Cert.Description
}

func (c *cmdConfigTrustList) issueDateColumnData(rowData rowData) string {
	return rowData.TLSCert.NotBefore.Local().Format(dateLayout)
}

func (c *cmdConfigTrustList) expiryDateColumnData(rowData rowData) string {
	return rowData.TLSCert.NotAfter.Local().Format(dateLayout)
}

func (c *cmdConfigTrustList) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	// Process the columns
	columns, err := c.parseColumns()
	if err != nil {
		return err
	}

	// List trust relationships
	resp, err := c.global.doHTTPRequestV1("/certificates", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	trust := []api.Certificate{}

	err = responseToStruct(resp, &trust)
	if err != nil {
		return err
	}

	data := [][]string{}
	for _, cert := range trust {
		certBlock, _ := pem.Decode([]byte(cert.Certificate))
		if certBlock == nil {
			return fmt.Errorf("Invalid certificate")
		}

		tlsCert, err := x509.ParseCertificate(certBlock.Bytes)
		if err != nil {
			return err
		}

		rowData := rowData{cert, tlsCert}

		row := []string{}
		for _, column := range columns {
			row = append(row, column.Data(rowData))
		}

		data = append(data, row)
	}

	sort.Sort(internalUtil.StringList(data))

	headers := []string{}
	for _, column := range columns {
		headers = append(headers, column.Name)
	}

	return internalUtil.RenderTable(os.Stdout, c.flagFormat, headers, data, trust)
}

// Remove.
type cmdConfigTrustRemove struct {
	global *CmdGlobal
	config *CmdConfig
}

func (c *cmdConfigTrustRemove) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "remove <fingerprint>"
	cmd.Short = "Remove trusted client"
	cmd.Long = `Description:
  Remove trusted client
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigTrustRemove) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	fingerprint := args[0]

	// Remove trust relationship
	_, err = c.global.doHTTPRequestV1("/certificates/"+url.PathEscape(fingerprint), http.MethodDelete, "", nil)

	return err
}

// Show.
type cmdConfigTrustShow struct {
	global *CmdGlobal
	config *CmdConfig
}

func (c *cmdConfigTrustShow) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "show <fingerprint>"
	cmd.Short = "Show trust configurations"
	cmd.Long = `Description:
  Show trust configurations
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdConfigTrustShow) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	fingerprint := args[0]

	// Show the certificate configuration
	resp, err := c.global.doHTTPRequestV1("/certificates/"+url.PathEscape(fingerprint), http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	cert := api.Certificate{}

	err = responseToStruct(resp, &cert)
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(&cert)
	if err != nil {
		return err
	}

	fmt.Printf("%s", data)

	return nil
}
