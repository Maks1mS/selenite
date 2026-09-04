// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package patch

import (
	"testing"

	"github.com/samber/mo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "altlinux.space/alt-atomic/selenite/internal/machine/config/v1"
)

func baseConfig() v1.MachineConfig {
	return v1.MachineConfig{
		Sysctl: v1.SysctlConfig{
			Params: map[string]string{
				"vm.swappiness": "60",
			},
		},
		Access: v1.AccessConfig{
			Users: []v1.User{
				{Name: "admin", Roles: []string{"admin"}},
			},
			Roles: []v1.Role{
				{Name: "admin", Rules: []v1.Rules{{Resources: []string{"*"}, Verbs: []string{"*"}}}},
			},
			AuthGuard: v1.AuthGuardConfig{
				Enabled:    mo.Some(false),
				MaxRetries: mo.Some(3),
			},
		},
		Netbird: v1.NetbirdConfig{
			Enabled: mo.Some(false),
		},
		Memory: v1.MemoryConfig{
			Zram: v1.ZramConfig{
				Enabled: mo.Some(true),
				Size:    mo.Some("1G"),
			},
		},
	}
}

func TestPatchNoPatchesReturnsSameConfig(t *testing.T) {
	before := baseConfig()
	after, err := Patch(&before, nil)
	require.NoError(t, err)
	assert.Equal(t, after, &before)
}

func TestPatch_MergePatch_YAML_AddAndOverride(t *testing.T) {
	before := baseConfig()

	patch := []byte(`
sysctl:
  params:
    vm.max_map_count: "262144"
netbird:
  enabled: true
  setup_key: "abc123"
`)

	after, err := Patch(&before, [][]byte{patch})
	require.NoError(t, err)

	assert.Equal(t, "262144", after.Sysctl.Params["vm.max_map_count"])
	// RFC 7396 merges objects recursively, so the original key must survive.
	assert.Equal(t, "60", after.Sysctl.Params["vm.swappiness"])

	enabled, ok := after.Netbird.Enabled.Get()
	require.True(t, ok, "Netbird.Enabled should be present")
	assert.True(t, enabled)

	setupKey, ok := after.Netbird.SetupKey.Get()
	require.True(t, ok, "Netbird.SetupKey should be present")
	assert.Equal(t, "abc123", setupKey)
}

func TestPatch_MergePatch_YAML_NullRemovesField(t *testing.T) {
	before := baseConfig()

	patch := []byte(`
access:
  auth_guard:
    max_retries: null
`)

	after, err := Patch(&before, [][]byte{patch})
	require.NoError(t, err)

	assert.False(t, after.Access.AuthGuard.MaxRetries.IsPresent())

	authEnabled, ok := after.Access.AuthGuard.Enabled.Get()
	require.True(t, ok)
	assert.False(t, authEnabled)
}

func TestPatch_MergePatch_YAML_TildeIsNull(t *testing.T) {
	// YAML "~" is also null and should behave the same as an explicit "null".
	before := baseConfig()

	patch := []byte(`
memory:
  zram:
    size: ~
`)

	after, err := Patch(&before, [][]byte{patch})
	require.NoError(t, err)

	assert.False(t, after.Memory.Zram.Size.IsPresent())
}

func TestPatch_JSONPatch_YAML_ReplaceAndAdd(t *testing.T) {
	before := baseConfig()

	patch := []byte(`
- op: replace
  path: /access/auth_guard/enabled
  value: true
- op: add
  path: /sysctl/params/net.ipv4.ip_forward
  value: "1"
`)

	after, err := Patch(&before, [][]byte{patch})
	require.NoError(t, err)

	authEnabled, ok := after.Access.AuthGuard.Enabled.Get()
	require.True(t, ok)
	assert.True(t, authEnabled)

	assert.Equal(t, "1", after.Sysctl.Params["net.ipv4.ip_forward"])
	assert.Equal(t, "60", after.Sysctl.Params["vm.swappiness"])
}

func TestPatch_JSONPatch_YAML_AppendToArray(t *testing.T) {
	before := baseConfig()

	patch := []byte(`
- op: add
  path: /access/users/-
  value:
    Name: bob
    Roles: [viewer]
`)

	after, err := Patch(&before, [][]byte{patch})
	require.NoError(t, err)

	require.Len(t, after.Access.Users, 2)
	assert.Equal(t, "admin", after.Access.Users[0].Name)
	assert.Equal(t, "bob", after.Access.Users[1].Name)
	assert.Equal(t, []string{"viewer"}, after.Access.Users[1].Roles)
}

func TestPatch_JSONPatch_YAML_Remove(t *testing.T) {
	before := baseConfig()

	patch := []byte(`
- op: remove
  path: /netbird/enabled
`)

	after, err := Patch(&before, [][]byte{patch})
	require.NoError(t, err)

	assert.False(t, after.Netbird.Enabled.IsPresent())
}

func TestPatch_MultiplePatches_YAML_AppliedSequentially(t *testing.T) {
	before := baseConfig()

	patch1 := []byte(`
memory:
  zram:
    enabled: true
`)
	patch2 := []byte(`
- op: replace
  path: /memory/zram/size
  value: "2G"
`)
	patch3 := []byte(`
memory:
  zram:
    compression: zstd
`)

	after, err := Patch(&before, [][]byte{patch1, patch2, patch3})
	require.NoError(t, err)

	zramEnabled, ok := after.Memory.Zram.Enabled.Get()
	require.True(t, ok)
	assert.True(t, zramEnabled)

	size, ok := after.Memory.Zram.Size.Get()
	require.True(t, ok)
	assert.Equal(t, "2G", size)

	compression, ok := after.Memory.Zram.Compression.Get()
	require.True(t, ok)
	assert.Equal(t, "zstd", compression)
}

/*


// --- Merge Patch (RFC 7396), written in YAML: the expected real-world format ---


// --- Multiple patches applied sequentially, all YAML ---


// --- Raw JSON input should still work, since it's valid YAML too ---

func TestPatch_RawJSONInput_StillWorks(t *testing.T) {
	cfg := baseConfig()

	patch := []byte(`{"netbird":{"enabled":true}}`)

	out, err := Patch(cfg, [][]byte{patch})
	require.NoError(t, err)

	got := unmarshalResult(t, out)
	enabled, ok := got.Netbird.Enabled.Get()
	require.True(t, ok)
	assert.True(t, enabled)
}

func TestPatch_InvalidPatch_NotObjectOrArray(t *testing.T) {
	cfg := baseConfig()

	patch := []byte(`"just a plain string"`)

	_, err := Patch(cfg, [][]byte{patch})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "incorrect patch")
}

func TestPatch_InvalidYAML(t *testing.T) {
	cfg := baseConfig()

	// Tabs are not valid YAML indentation.
	patch := []byte("foo:\n\tbar: 1\n")

	_, err := Patch(cfg, [][]byte{patch})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "yaml.YAMLToJSON")
}

func TestPatch_JSONPatch_DecodeError(t *testing.T) {
	cfg := baseConfig()

	// A YAML/JSON list whose elements aren't patch operation objects.
	patch := []byte(`[1, 2, 3]`)

	_, err := Patch(cfg, [][]byte{patch})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jsonpatch.DecodePatch")
}

func TestPatch_JSONPatch_ApplyError(t *testing.T) {
	cfg := baseConfig()

	// The "test" op fails because the actual value differs, so Apply errors.
	patch := []byte(`
- op: test
  path: /sysctl/params/vm.swappiness
  value: "999"
`)

	_, err := Patch(cfg, [][]byte{patch})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decodedPatch.Apply")
}

func TestPatch_StopsOnFirstError(t *testing.T) {
	cfg := baseConfig()

	good := []byte(`
netbird:
  enabled: true
`)
	bad := []byte("not: [valid")

	_, err := Patch(cfg, [][]byte{good, bad})
	require.Error(t, err)
}

// unmarshalResult decodes the JSON bytes returned by Patch back into a
// MachineConfig using the standard encoding/json package (the same
// unmarshaling path Patch relies on internally via mo.Option's
// MarshalJSON/UnmarshalJSON).
func unmarshalResult(t *testing.T, b []byte) *v1.MachineConfig {
	t.Helper()
	var cfg v1.MachineConfig
	require.NoError(t, json.Unmarshal(b, &cfg), "raw: %s", string(b))
	return &cfg
}

// rawField unmarshals the top-level Patch output into a generic map, useful
// for asserting that a key was removed entirely (rather than set to a zero
// value), which unmarshalResult can't distinguish.
func rawField(t *testing.T, b []byte, path ...string) map[string]json.RawMessage {
	t.Helper()
	cur := map[string]json.RawMessage{"root": b}
	key := "root"
	for _, p := range path {
		var m map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(cur[key], &m))
		cur = m
		key = p
	}
	var final map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(cur[key], &final))
	return final
}
*/
