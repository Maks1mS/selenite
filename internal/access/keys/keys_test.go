package keys

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestKeysManager(t *testing.T) *keysManager {
	t.Helper()
	return &keysManager{path.Join(t.TempDir(), "keys")}
}

func TestParseComment(t *testing.T) {
	k, err := parseComment("user;description")
	assert.NoError(t, err)
	assert.Equal(t, k.Name, "user")
	assert.Equal(t, k.Comment, "description")

	k, err = parseComment("user2")
	assert.NoError(t, err)
	assert.Equal(t, k.Name, "user2")
	assert.Equal(t, k.Comment, "")

	k, err = parseComment(";")
	assert.Error(t, err)
	assert.Equal(t, k.Name, "")
	assert.Equal(t, k.Comment, "")

	k, err = parseComment("")
	assert.Error(t, err)
	assert.Equal(t, k.Name, "")
	assert.Equal(t, k.Comment, "")
}

func TestParseKeyFileContent(t *testing.T) {
	content := []byte(
		`ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHCChbrR+bpkXBKFgBsjgZO1RvLtGxIDRQV3zEJSa4B5 user;very important user
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIN3Oc+LIc+YCCDe2k2EWSOpK2Ulx8ttzSbT26fAYwLM2 user2;awesome user
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILxWe2rXKoiO6W14LYPVfJKzRfJ1f3Jhzxrgjc/D4tU7 user3;default user`,
	)

	keys, err := parseKeyFileContent(content)
	assert.NoError(t, err)

	assert.Len(t, keys, 3)
	assert.Equal(t, keys[0].Name, "user")
	assert.Equal(t, keys[0].Comment, "very important user")
	assert.Equal(t, keys[1].Name, "user2")
	assert.Equal(t, keys[1].Comment, "awesome user")
	assert.Equal(t, keys[2].Name, "user3")
	assert.Equal(t, keys[2].Comment, "default user")
}

func TestKeysList(t *testing.T) {
	m := newTestKeysManager(t)

	keyStr := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILxWe2rXKoiO6W14LYPVfJKzRfJ1f3Jhzxrgjc/D4tU7 user;very important user"

	require.NoError(
		t,
		os.WriteFile(
			m.keysFile,
			[]byte(keyStr+"\n"),
			0o600,
		),
	)

	keys, err := m.List()
	assert.NoError(t, err)
	assert.Len(t, keys, 1)
	assert.Equal(t, keys[0].Name, "user")
	assert.Equal(t, keys[0].Comment, "very important user")
	assert.Equal(t, keys[0].ToString(), keyStr)
}

func TestKeyAdd(t *testing.T) {
	m := newTestKeysManager(t)
	require.NoError(t, os.WriteFile(m.keysFile, []byte(""), 0o600))

	key1 := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILxWe2rXKoiO6W14LYPVfJKzRfJ1f3Jhzxrgjc/D4tU7"
	user1 := "user;very important user"
	user2 := "user2"

	fullKey := fmt.Sprintf("%s %s", key1, user1)

	err := m.Add(fullKey)
	require.NoError(t, err)

	b, err := os.ReadFile(m.keysFile)
	require.NoError(t, err)
	require.Equal(t, fullKey+"\n", string(b))

	err = m.Add(fmt.Sprintf("%s %s", key1, user1))
	require.ErrorIs(t, err, errKeyWithSameName)

	err = m.Add(fmt.Sprintf("%s %s", key1, user2))
	require.ErrorIs(t, err, errKeyAlreadyExist)
}

func TestKeyRemove(t *testing.T) {
	m := newTestKeysManager(t)
	require.NoError(t, os.WriteFile(m.keysFile, []byte(""), 0o600))

	fullKey1 := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILxWe2rXKoiO6W14LYPVfJKzRfJ1f3Jhzxrgjc/D4tU7 user;very important user"
	fullKey2 := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIN3Oc+LIc+YCCDe2k2EWSOpK2Ulx8ttzSbT26fAYwLM2 user2;awesome user"
	require.NoError(
		t,
		os.WriteFile(
			m.keysFile,
			[]byte(fullKey1+"\n"+fullKey2),
			0o600,
		),
	)

	require.NoError(t, m.Remove("user"))

	b, err := os.ReadFile(m.keysFile)
	require.NoError(t, err)
	require.Equal(t, fullKey2+"\n", string(b))
}

func TestCompound(t *testing.T) {
	m := newTestKeysManager(t)
	require.NoError(t, os.WriteFile(m.keysFile, []byte(""), 0o600))

	fullKey1 := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILxWe2rXKoiO6W14LYPVfJKzRfJ1f3Jhzxrgjc/D4tU7 user;very important user"
	fullKey2 := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIN3Oc+LIc+YCCDe2k2EWSOpK2Ulx8ttzSbT26fAYwLM2 user2;awesome user"

	require.NoError(
		t,
		os.WriteFile(
			m.keysFile,
			[]byte(fullKey1+"\n"),
			0o600,
		),
	)

	keys, err := m.List()
	assert.NoError(t, err)
	assert.Len(t, keys, 1)
	assert.Equal(t, keys[0].Name, "user")
	assert.Equal(t, keys[0].Comment, "very important user")

	err = m.Add(fullKey2)
	assert.NoError(t, err)

	keys, err = m.List()
	assert.NoError(t, err)
	assert.Len(t, keys, 2)

	err = m.Remove("user")
	assert.NoError(t, err)

	keys, err = m.List()
	assert.NoError(t, err)
	assert.Len(t, keys, 1)
}
