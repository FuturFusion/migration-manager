package api

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/FuturFusion/migration-manager/internal/ptr"
)

type RecursiveA struct {
	B []RecursiveB `yaml:"b"`
}

type RecursiveB struct {
	C []RecursiveC `yaml:"c"`
}

type RecursiveC struct {
	A []RecursiveA `yaml:"a"`
}

func TestParseExpr(t *testing.T) {
	type SubObj struct {
		SubName string    `yaml:"sub_name,omitempty"`
		Time    time.Time `yaml:"sub_time,omitempty"`
	}

	type Obj struct {
		SubObj

		Name    string
		NilName *string

		Names    []string
		NilNames *[]*string

		Map    map[string]SubObj
		NilMap *map[*string]*SubObj

		Time time.Time
		UUID uuid.UUID

		NilTime *time.Time
		NilUUID *uuid.UUID
	}

	type ObjTags struct {
		SubObj `yaml:",inline"`

		Name    string  `yaml:"name"`
		NilName *string `yaml:"nil_name"`

		Names    []string   `yaml:"names"`
		NilNames *[]*string `yaml:"nil_names"`

		Map    map[string]SubObj    `yaml:"map"`
		NilMap *map[*string]*SubObj `yaml:"nil_map"`

		Time time.Time `yaml:"time"`
		UUID uuid.UUID `yaml:"uuid"`

		NilTime *time.Time `yaml:"nil_time"`
		NilUUID *uuid.UUID `yaml:"nil_uuid"`

		Omit string `yaml:"-"`
	}

	type ObjOmitTags struct {
		SubObj `yaml:",inline,omitempty"`

		Name    string  `yaml:"name,omitempty"`
		NilName *string `yaml:"nil_name,omitempty"`

		Names    []string   `yaml:"names,omitempty"`
		NilNames *[]*string `yaml:"nil_names,omitempty"`

		Map    map[string]SubObj    `yaml:"map,omitempty"`
		NilMap *map[*string]*SubObj `yaml:"nil_map,omitempty"`

		Time time.Time `yaml:"time,omitempty"`
		UUID uuid.UUID `yaml:"uuid,omitempty"`

		NilTime *time.Time `yaml:"nil_time,omitempty"`
		NilUUID *uuid.UUID `yaml:"nil_uuid,omitempty"`

		Omit string `yaml:"-"`
	}

	parseTime := func(now time.Time) time.Time {
		b, err := yaml.Marshal(now)
		require.NoError(t, err)

		var newtime time.Time
		require.NoError(t, yaml.Unmarshal(b, &newtime))

		return newtime
	}

	// Yaml marshaling strips some time.Time data so parse the values once beforehand.
	now1 := parseTime(time.Now())
	now2 := parseTime(time.Now())
	now3 := parseTime(time.Now())
	now4 := parseTime(time.Now())
	uuid1 := uuid.New()

	cases := []struct {
		name      string
		obj       any
		result    map[string]any
		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "success - no tags, populated",
			obj: Obj{
				SubObj: SubObj{
					SubName: "s1",
					Time:    now1,
				},
				Name:     "n1",
				NilName:  ptr.To("n2"),
				Names:    []string{"n3"},
				NilNames: &[]*string{ptr.To("n4")},
				Map:      map[string]SubObj{"k1": {SubName: "s2", Time: now2}},
				NilMap:   &map[*string]*SubObj{ptr.To("k2"): {SubName: "s3", Time: now3}},
				Time:     now4,
				UUID:     uuid1,
				NilTime:  &now4,
				NilUUID:  &uuid1,
			},
			result: map[string]any{
				"subobj": map[string]any{"sub_name": "s1", "sub_time": now1},
				"name":   "n1", "nilname": "n2", "names": []any{"n3"}, "nilnames": []any{"n4"},
				"map":    map[string]any{"k1": map[string]any{"sub_name": "s2", "sub_time": now2}},
				"nilmap": map[string]any{"k2": map[string]any{"sub_name": "s3", "sub_time": now3}},
				"time":   now4, "uuid": uuid1.String(), "niltime": now4, "niluuid": uuid1.String(),
			},
			assertErr: require.NoError,
		},
		{
			name: "success - no tags, unpopulated",
			obj:  Obj{},
			result: map[string]any{
				"subobj": map[string]any{}, "name": "", "nilname": nil, "names": []any{}, "nilnames": nil,
				"map": map[string]any{}, "nilmap": nil, "time": time.Time{}, "uuid": uuid.Nil.String(),
				"niltime": nil, "niluuid": nil,
			},
			assertErr: require.NoError,
		},
		{
			name: "success - YAML tags, populated",
			obj: ObjTags{
				SubObj: SubObj{
					SubName: "s1",
					Time:    now1,
				},
				Name:     "n1",
				NilName:  ptr.To("n2"),
				Names:    []string{"n3"},
				NilNames: &[]*string{ptr.To("n4")},
				Map:      map[string]SubObj{"k1": {SubName: "s2", Time: now2}},
				NilMap:   &map[*string]*SubObj{ptr.To("k2"): {SubName: "s3", Time: now3}},
				Time:     now4,
				UUID:     uuid1,
				NilTime:  &now4,
				NilUUID:  &uuid1,
				Omit:     "omit",
			},
			result: map[string]any{
				"sub_name": "s1", "sub_time": now1,
				"name": "n1", "nil_name": "n2", "names": []any{"n3"}, "nil_names": []any{"n4"},
				"map":     map[string]any{"k1": map[string]any{"sub_name": "s2", "sub_time": now2}},
				"nil_map": map[string]any{"k2": map[string]any{"sub_name": "s3", "sub_time": now3}},
				"time":    now4, "uuid": uuid1.String(), "nil_time": now4, "nil_uuid": uuid1.String(),
			},
			assertErr: require.NoError,
		},
		{
			name: "success - YAML tags, unpopulated",
			obj:  ObjTags{},
			result: map[string]any{
				"sub_name": "", "sub_time": time.Time{},
				"name": "", "nil_name": nil, "names": []any{}, "nil_names": nil,
				"map": map[string]any{}, "nil_map": nil, "time": time.Time{}, "uuid": uuid.Nil.String(),
				"nil_time": nil, "nil_uuid": nil,
			},
			assertErr: require.NoError,
		},
		{
			name: "success - YAML omit tags, populated",
			obj: ObjOmitTags{
				SubObj: SubObj{
					SubName: "s1",
					Time:    now1,
				},
				Name:     "n1",
				NilName:  ptr.To("n2"),
				Names:    []string{"n3"},
				NilNames: &[]*string{ptr.To("n4")},
				Map:      map[string]SubObj{"k1": {SubName: "s2", Time: now2}},
				NilMap:   &map[*string]*SubObj{ptr.To("k2"): {SubName: "s3", Time: now3}},
				Time:     now4,
				UUID:     uuid1,
				NilTime:  &now4,
				NilUUID:  &uuid1,
				Omit:     "omit",
			},
			result: map[string]any{
				"sub_name": "s1", "sub_time": now1,
				"name": "n1", "nil_name": "n2", "names": []any{"n3"}, "nil_names": []any{"n4"},
				"map":     map[string]any{"k1": map[string]any{"sub_name": "s2", "sub_time": now2}},
				"nil_map": map[string]any{"k2": map[string]any{"sub_name": "s3", "sub_time": now3}},
				"time":    now4, "uuid": uuid1.String(), "nil_time": now4, "nil_uuid": uuid1.String(),
			},
			assertErr: require.NoError,
		},
		{
			name: "success - YAML omit tags, unpopulated",
			obj:  ObjOmitTags{},
			result: map[string]any{
				"sub_name": "", "sub_time": time.Time{},
				"name": "", "nil_name": nil, "names": []any{}, "nil_names": nil,
				"map": map[string]any{}, "nil_map": nil, "time": time.Time{}, "uuid": uuid.Nil.String(), "nil_uuid": nil,
			},
			assertErr: require.NoError,
		},
		{
			name:      "error - supplied object is nil",
			assertErr: require.Error,
		},
		{
			name: "error - supplied object has recursive fields",
			obj: func() any {
				type A struct {
					A *A `yaml:"a"`
				}

				return A{}
			}(),
			assertErr: require.Error,
		},
		{
			name:      "error - supplied object has nested recursive fields",
			obj:       RecursiveA{},
			assertErr: require.Error,
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)

			out, err := ParseExpr(tc.obj)
			tc.assertErr(t, err)
			require.Equal(t, tc.result, out)

			// Ensure they are the same post-parse.
			if out != nil {
				b1, err := yaml.Marshal(tc.result)
				require.NoError(t, err)
				b2, err := yaml.Marshal(out)
				require.NoError(t, err)
				require.Equal(t, string(b1), string(b2))
			}
		})
	}
}
