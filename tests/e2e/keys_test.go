//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeys(t *testing.T) {
	t.Parallel()
	client := bootVMWithProvison(t)

	run(t, "keys add", func(t *testing.T) {
		keyStr := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILxWe2rXKoiO6W14LYPVfJKzRfJ1f3Jhzxrgjc/D4tU7 user;very important user"

		out := mustRunCmdWithStdin(t, client, "access keys add", strings.NewReader(keyStr))
		assert.Equal(t, "", out)
		out = mustRunCmd(t, client, "access keys list")
		assert.Equal(t, client.FormatKey()+"\n"+keyStr+"\n", out)
	})

	run(t, "keys remove", func(t *testing.T) {
		out := mustRunCmd(t, client, "access keys remove user")
		assert.Equal(t, "", out)
		out = mustRunCmd(t, client, "access keys list")
		assert.Equal(t, client.FormatKey()+"\n", out)
	})
}
