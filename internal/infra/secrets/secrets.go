// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package secrets

import (
	"context"
	"fmt"
	"io"

	"altlinux.space/alt-atomic/selenite/internal/executil"
)

const (
	ExecutorBin = "/usr/libexec/selenite-controller/podman-secret-helper"
)

type secretsManager struct {
	runner executil.Runner
}

func New() *secretsManager {
	return &secretsManager{
		runner: executil.ExecRunner{},
	}
}

func (m *secretsManager) run(ctx context.Context, w io.Writer, args ...string) error {
	if err := m.runner.Run(ctx, ExecutorBin, args, executil.WithCombinedOutput(w)); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return nil
}

func (m *secretsManager) List(ctx context.Context, w io.Writer) error {
	err := m.run(ctx, w, "ls")
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}
	return nil
}

func (m *secretsManager) Create(ctx context.Context, name string, r io.Reader, w io.Writer) error {
	if err := m.runner.Run(
		ctx,
		ExecutorBin,
		[]string{"create", name, "-"},
		executil.WithStdin(r),
		executil.WithStdout(w),
	); err != nil {
		return fmt.Errorf("create: %w", err)
	}
	return nil
}

func (m *secretsManager) Rm(ctx context.Context, name string, w io.Writer) error {
	err := m.run(ctx, w, "rm", name)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}
	return nil
}
