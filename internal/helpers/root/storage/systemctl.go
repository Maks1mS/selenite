// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"strings"

	"altlinux.space/alt-atomic/selenite/internal/executil"
	"altlinux.space/alt-atomic/selenite/internal/privdrop"
)

// Systemctl abstracts the systemctl(1) operations Manager needs.
type Systemctl interface {
	DaemonReload(ctx context.Context) error
	ExecutorDaemonReload(ctx context.Context) error
	Enable(ctx context.Context, unit string) error
	Disable(ctx context.Context, unit string) error
	Start(ctx context.Context, unit string) error
	Restart(ctx context.Context, unit string) error
	Stop(ctx context.Context, unit string) error
	IsActive(ctx context.Context, unit string) bool
}

type HostSystemctl struct{}

func systemctl(ctx context.Context, args ...string) error {
	runner := executil.ExecRunner{}
	if err := runner.Run(ctx, "/usr/bin/systemctl", args,
		executil.WithStdout(os.Stdout),
		executil.WithStderr(os.Stderr),
	); err != nil {
		return fmt.Errorf("systemctl %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func (HostSystemctl) DaemonReload(ctx context.Context) error {
	return systemctl(ctx, "daemon-reload")
}

func (HostSystemctl) Enable(ctx context.Context, unit string) error {
	return systemctl(ctx, "enable", unit)
}

func (HostSystemctl) Disable(ctx context.Context, unit string) error {
	return systemctl(ctx, "disable", unit)
}

func (HostSystemctl) Start(ctx context.Context, unit string) error {
	return systemctl(ctx, "start", unit)
}

func (HostSystemctl) Restart(ctx context.Context, unit string) error {
	return systemctl(ctx, "restart", unit)
}

func (HostSystemctl) Stop(ctx context.Context, unit string) error {
	return systemctl(ctx, "stop", unit)
}

func (HostSystemctl) IsActive(ctx context.Context, unit string) bool {
	return systemctl(ctx, "is-active", "-q", unit) == nil
}

func (HostSystemctl) ExecutorDaemonReload(ctx context.Context) error {
	podmanUser := privdrop.PodmanExecutorUser

	u, err := user.Lookup(podmanUser)
	if err != nil {
		return fmt.Errorf("lookup %s: %w", podmanUser, err)
	}
	runner := executil.ExecRunner{}
	if err := runner.Run(ctx, "runuser",
		[]string{"-u", podmanUser, "--", "systemctl", "--user", "daemon-reload"},
		executil.WithEnv("XDG_RUNTIME_DIR", "/run/user/"+u.Uid),
		executil.WithStdout(os.Stdout),
		executil.WithStderr(os.Stderr),
	); err != nil {
		return fmt.Errorf("user daemon-reload: %w", err)
	}
	return nil
}
