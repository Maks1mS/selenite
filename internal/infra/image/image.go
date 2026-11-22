// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package image manages container images via the podman image helper.
package image

import (
	"context"
	"fmt"
	"io"

	"altlinux.space/alt-atomic/selenite/internal/executil"
)

const (
	ExecutorBin = "/usr/libexec/selenite-controller/podman-image-helper"
)

type imageManager struct {
	runner executil.Runner
}

func New() *imageManager {
	return &imageManager{
		runner: executil.ExecRunner{},
	}
}

// Load reads a container image archive from r and loads it into the local
// image storage, writing progress to w.
func (m *imageManager) Load(ctx context.Context, r io.Reader, w io.Writer) error {
	if err := m.runner.Run(
		ctx,
		ExecutorBin,
		[]string{"load"},
		executil.WithStdin(r),
		executil.WithCombinedOutput(w),
	); err != nil {
		return fmt.Errorf("image load: %w", err)
	}
	return nil
}

// List writes the local container images to w.
func (m *imageManager) List(ctx context.Context, w io.Writer) error {
	if err := m.runner.Run(
		ctx,
		ExecutorBin,
		[]string{"list"},
		executil.WithCombinedOutput(w),
	); err != nil {
		return fmt.Errorf("image list: %w", err)
	}
	return nil
}

// Remove deletes the named container image.
func (m *imageManager) Remove(ctx context.Context, name string, w io.Writer) error {
	if err := m.runner.Run(
		ctx,
		ExecutorBin,
		[]string{"rm", name},
		executil.WithCombinedOutput(w),
	); err != nil {
		return fmt.Errorf("image remove: %w", err)
	}
	return nil
}

// Prune removes all unused container images.
func (m *imageManager) Prune(ctx context.Context, w io.Writer) error {
	if err := m.runner.Run(
		ctx,
		ExecutorBin,
		[]string{"prune", "-f"},
		executil.WithCombinedOutput(w),
	); err != nil {
		return fmt.Errorf("image prune: %w", err)
	}
	return nil
}
