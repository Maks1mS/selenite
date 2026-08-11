// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package debug

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"charm.land/ssh"
)

const HelperBin = "/usr/libexec/selenite-controller/debug-helper"

func Run(ctx context.Context, sess ssh.Session, image string, args []string) error {
	ppty, _, _ := sess.Pty()

	cmdArgs := append([]string{image}, args...)
	//nolint:gosec // HelperBin is a const, args are passed through to the trusted helper
	cmd := exec.CommandContext(ctx, HelperBin, cmdArgs...)
	env := os.Environ()
	env = appendOrReplace(env, "TERM="+ppty.Term)
	cmd.Env = env

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    0,
	}

	if err := ppty.Start(cmd); err != nil {
		return fmt.Errorf("failed to start: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to wait: %w", err)
	}
	return nil
}

func appendOrReplace(env []string, kv string) []string {
	key := strings.SplitN(kv, "=", 2)[0] + "="
	for i, e := range env {
		if strings.HasPrefix(e, key) {
			env[i] = kv
			return env
		}
	}
	return append(env, kv)
}
