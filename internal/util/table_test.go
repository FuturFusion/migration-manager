package util_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var headers = []string{
	"Name", "Type", "Endpoint", "Username", "Insecure",
}

var entries = [][]string{
	{
		"source 1",
		"VMware",
		"https://127.0.0.1:8989/",
		"user",
		"false",
	},
	{
		"source 2",
		"Incus",
		"https://127.0.0.2:8989/",
		"user2",
		"true",
	},
	{
		"source 3",
		"Other",
		"https://127.0.0.3:8989/",
		"user3",
		"false",
	},
}

var raw = []api.VMwareSource{
	{
		CommonSource: api.CommonSource{
			Name:       "source 1",
			DatabaseID: 1,
			Insecure:   false,
		},
		VMwareSourceSpecific: api.VMwareSourceSpecific{
			Endpoint: "https://127.0.0.1:8989/",
			Username: "user",
			Password: "pass",
		},
	},
	{
		CommonSource: api.CommonSource{
			Name:       "source 2",
			DatabaseID: 2,
			Insecure:   true,
		},
		VMwareSourceSpecific: api.VMwareSourceSpecific{
			Endpoint: "https://127.0.0.2:8989/",
			Username: "user2",
			Password: "pass2",
		},
	},
	{
		CommonSource: api.CommonSource{
			Name:       "source 3",
			DatabaseID: 3,
			Insecure:   false,
		},
		VMwareSourceSpecific: api.VMwareSourceSpecific{
			Endpoint: "https://127.0.0.3:8989/",
			Username: "user3",
			Password: "pass3",
		},
	},
}

func TestRenderTable(t *testing.T) {
	tests := []struct {
		name   string
		format string

		assertErr             require.ErrorAssertionFunc
		wantOutputContains    []string
		wantOutputNotContains []string
		wantJSONEQ            []string
	}{
		{
			name:   "success - table",
			format: `table`,

			assertErr: require.NoError,
			wantOutputContains: []string{
				`|   NAME   |  TYPE  |        ENDPOINT         | USERNAME | INSECURE |`,
				`| source 1 | VMware | https://127.0.0.1:8989/ | user     | false    |`,
				`| source 2 | Incus  | https://127.0.0.2:8989/ | user2    | true     |`,
				`| source 3 | Other  | https://127.0.0.3:8989/ | user3    | false    |`,
			},
		},
		{
			name:   "success - table without header",
			format: `table,noheader`,

			assertErr: require.NoError,
			wantOutputContains: []string{
				`| source 1 | VMware | https://127.0.0.1:8989/ | user  | false |`,
				`| source 2 | Incus  | https://127.0.0.2:8989/ | user2 | true  |`,
				`| source 3 | Other  | https://127.0.0.3:8989/ | user3 | false |`,
			},
			wantOutputNotContains: []string{
				`NAME`,
				`TYPE`,
				`ENDPOINT`,
				`USERNAME`,
				`INSECURE`,
			},
		},
		{
			name:   "success - csv",
			format: "csv",

			assertErr: require.NoError,
			wantOutputContains: []string{
				`Name,Type,Endpoint,Username,Insecure`,
				`source 1,VMware,https://127.0.0.1:8989/,user,false`,
				`source 2,Incus,https://127.0.0.2:8989/,user2,true`,
				`source 3,Other,https://127.0.0.3:8989/,user3,false`,
			},
		},
		{
			name:   "success - csv without header",
			format: "csv,noheader",

			assertErr: require.NoError,
			wantOutputContains: []string{
				`source 1,VMware,https://127.0.0.1:8989/,user,false`,
				`source 2,Incus,https://127.0.0.2:8989/,user2,true`,
				`source 3,Other,https://127.0.0.3:8989/,user3,false`,
			},
			wantOutputNotContains: []string{
				`Name`,
				`Type`,
				`Endpoint`,
				`Username`,
				`Insecure`,
			},
		},
		{
			name:   "success - compact",
			format: `compact`,

			assertErr: require.NoError,
			wantOutputContains: []string{
				`NAME     TYPE          ENDPOINT          USERNAME  INSECURE`,
				`source 1  VMware  https://127.0.0.1:8989/  user      false`,
				`source 2  Incus   https://127.0.0.2:8989/  user2     true`,
				`source 3  Other   https://127.0.0.3:8989/  user3     false`,
			},
		},
		{
			name:   "success - list as compact without header",
			format: `compact,noheader`,

			assertErr: require.NoError,
			wantOutputContains: []string{
				`source 1  VMware  https://127.0.0.1:8989/  user   false`,
				`source 2  Incus   https://127.0.0.2:8989/  user2  true`,
				`source 3  Other   https://127.0.0.3:8989/  user3  false`,
			},
		},
		{
			name:   "success - list as json",
			format: `json`,

			assertErr: require.NoError,
			wantJSONEQ: []string{
				`[
  {
    "name": "source 1",
    "database_id": 1,
    "insecure": false,
    "endpoint": "https://127.0.0.1:8989/",
    "username": "user",
    "password": "pass"
  },
  {
    "name": "source 2",
    "database_id": 2,
    "insecure": true,
    "endpoint": "https://127.0.0.2:8989/",
    "username": "user2",
    "password": "pass2"
  },
  {
    "name": "source 3",
    "database_id": 3,
    "insecure": false,
    "endpoint": "https://127.0.0.3:8989/",
    "username": "user3",
    "password": "pass3"
  }
]`,
			},
		},
		{
			name:   "success - list as yaml",
			format: `yaml`,

			assertErr: require.NoError,
			wantOutputContains: []string{
				`- name: source 1`,
				`database_id: 1`,
				`insecure: false`,
				`endpoint: https://127.0.0.1:8989/`,
				`username: user`,
				`password: pass`,
				`- name: source 2`,
				`database_id: 2`,
				`insecure: true`,
				`endpoint: https://127.0.0.2:8989/`,
				`username: user2`,
				`password: pass2`,
				`- name: source 3`,
				`database_id: 3`,
				`insecure: false`,
				`endpoint: https://127.0.0.3:8989/`,
				`username: user3`,
				`password: pass3`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := bytes.Buffer{}

			err := util.RenderTable(&buf, tc.format, headers, entries, raw)
			tc.assertErr(t, err)

			if testing.Verbose() {
				t.Logf("\n%s", buf.String())
			}

			for _, want := range tc.wantOutputContains {
				require.Contains(t, buf.String(), want)
			}

			for _, want := range tc.wantOutputNotContains {
				require.NotContains(t, buf.String(), want)
			}

			for _, want := range tc.wantJSONEQ {
				require.JSONEq(t, want, buf.String())
			}
		})
	}
}

func TestRenderTableNilWriter(t *testing.T) {
	err := util.RenderTable(nil, "table", headers, entries, raw)
	require.Error(t, err)
}

func TestRenderTableError(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		headers []string
		entries [][]string
		raw     any

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:    "csv render error",
			format:  "csv",
			headers: []string{"head 1", "head 2"},
			entries: [][]string{
				{
					"entry 1.1",
				},
				{
					"entry 2.1",
					"entry 2.2",
					"entry 2.3",
				},
			},

			assertErr: require.Error,
		},
		{
			name:   "json encoding error",
			format: "json",
			raw:    func() {}, // func type can not be encoded to JSON.

			assertErr: require.Error,
		},
		{
			name:   "yaml encoding error",
			format: "yaml",
			raw:    errTextMarshaler{},

			assertErr: require.Error,
		},
		{
			name:   "invalid format",
			format: "invalid",

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := errWriter{}

			err := util.RenderTable(w, tc.format, tc.headers, tc.entries, tc.raw)
			tc.assertErr(t, err)
		})
	}
}

type errWriter struct{}

func (errWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("boom!")
}

type errTextMarshaler struct{}

func (errTextMarshaler) MarshalText() ([]byte, error) {
	return nil, errors.New("boom!")
}
