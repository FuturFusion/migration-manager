package migration_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestInstance_GetOSType(t *testing.T) {
	tests := []struct {
		name     string
		instance migration.Instance

		want api.OSType
	}{
		{
			name: "windows",
			instance: migration.Instance{
				Properties: api.InstanceProperties{OS: "Something Windows Something"},
			},

			want: api.OSTYPE_WINDOWS,
		},
		{
			name: "linux",
			instance: migration.Instance{
				Properties: api.InstanceProperties{OS: "ubuntu64Guest"},
			},

			want: api.OSTYPE_LINUX,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.instance.GetOSType(true)

			require.Equal(t, tc.want, got)
		})
	}
}

func TestInstance_SDNTagConfig(t *testing.T) {
	tests := []struct {
		name string
		tags []api.InstancePropertiesSDNTag

		want map[string]string
	}{
		{
			name: "no tags",
			tags: nil,

			want: map[string]string{},
		},
		{
			name: "tags sharing a scope are indexed",
			tags: []api.InstancePropertiesSDNTag{
				{Scope: "app", Tag: "web"},
				{Scope: "app", Tag: "api"},
				{Scope: "dtap", Tag: "prod"},
			},

			want: map[string]string{
				"user.sdn.tags.0.app":  "web",
				"user.sdn.tags.1.app":  "api",
				"user.sdn.tags.0.dtap": "prod",
			},
		},
		{
			name: "scopeless tags use the tag as scope",
			tags: []api.InstancePropertiesSDNTag{{Tag: "foo"}},

			want: map[string]string{"user.sdn.tags.0.foo": "foo"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			instance := migration.Instance{Properties: api.InstanceProperties{SDNTags: tc.tags}}

			require.Equal(t, tc.want, instance.SDNTagConfig())
		})
	}
}

func TestInstance_ApplyUpdatesSDNTags(t *testing.T) {
	instance := func(tags []api.InstancePropertiesSDNTag) migration.Instance {
		return migration.Instance{
			Properties: api.InstanceProperties{
				// Set to avoid the fallback architecture counting as an update.
				InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{Architecture: "x86_64"},
				SDNTags:                        tags,
			},
		}
	}

	tests := []struct {
		name     string
		recorded []api.InstancePropertiesSDNTag
		synced   []api.InstancePropertiesSDNTag

		want        []api.InstancePropertiesSDNTag
		wantUpdated bool
	}{
		{
			name:     "tags are updated",
			recorded: []api.InstancePropertiesSDNTag{{Scope: "app", Tag: "web"}},
			synced:   []api.InstancePropertiesSDNTag{{Scope: "app", Tag: "api"}},

			want:        []api.InstancePropertiesSDNTag{{Scope: "app", Tag: "api"}},
			wantUpdated: true,
		},
		{
			name:     "tags are removed when the sync reports none",
			recorded: []api.InstancePropertiesSDNTag{{Scope: "app", Tag: "web"}},
			synced:   []api.InstancePropertiesSDNTag{},

			want:        []api.InstancePropertiesSDNTag{},
			wantUpdated: true,
		},
		{
			name:     "tags are kept when the SDN manager didn't report",
			recorded: []api.InstancePropertiesSDNTag{{Scope: "app", Tag: "web"}},
			synced:   nil,

			want:        []api.InstancePropertiesSDNTag{{Scope: "app", Tag: "web"}},
			wantUpdated: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, updated := instance(tc.recorded).ApplyUpdates(instance(tc.synced))

			require.Equal(t, tc.want, got.Properties.SDNTags)
			require.Equal(t, tc.wantUpdated, updated)
		})
	}
}
