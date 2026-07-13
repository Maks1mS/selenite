// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"io"

	"altlinux.space/alt-atomic/selenite/internal/executil"
)

const (
	ExecutorBin = "/usr/libexec/selenite-controller/podman-network-helper"
)

type networkManager struct {
	runner executil.Runner
}

func New() *networkManager {
	return &networkManager{
		runner: executil.ExecRunner{},
	}
}

func (m *networkManager) run(ctx context.Context, w io.Writer, args ...string) error {
	if err := m.runner.Run(ctx, ExecutorBin, args, executil.WithCombinedOutput(w)); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return nil
}

func (m *networkManager) List(ctx context.Context, w io.Writer) error {
	err := m.run(ctx, w, "ls")
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}
	return nil
}

func (m *networkManager) Create(ctx context.Context, name string, opts CreateOptions, w io.Writer) error {
	args := []string{"create"}
	if opts.Driver != "" {
		args = append(args, "-d", opts.Driver)
	}
	if opts.Subnet != "" {
		args = append(args, "--subnet", opts.Subnet)
	}
	if opts.Gateway != "" {
		args = append(args, "--gateway", opts.Gateway)
	}
	if opts.Internal {
		args = append(args, "--internal")
	}
	if opts.IPv6 {
		args = append(args, "--ipv6")
	}
	args = append(args, name)

	if err := m.runner.Run(ctx, ExecutorBin, args, executil.WithCombinedOutput(w)); err != nil {
		return fmt.Errorf("create: %w", err)
	}
	return nil
}

func (m *networkManager) Rm(ctx context.Context, name string, w io.Writer) error {
	err := m.run(ctx, w, "rm", name)
	if err != nil {
		return fmt.Errorf("rm: %w", err)
	}
	return nil
}

func (m *networkManager) Inspect(ctx context.Context, name string, w io.Writer) error {
	err := m.run(ctx, w, "inspect", name)
	if err != nil {
		return fmt.Errorf("inspect: %w", err)
	}
	return nil
}

func (m *networkManager) Prune(ctx context.Context, w io.Writer) error {
	err := m.run(ctx, w, "prune", "-f")
	if err != nil {
		return fmt.Errorf("prune: %w", err)
	}
	return nil
}

type CreateOptions struct {
	Driver   string
	Subnet   string
	Gateway  string
	Internal bool
	IPv6     bool
}
