// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package volume

import (
	"context"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"

	"altlinux.space/alt-atomic-lab/selenite/internal/executil"
)

const (
	ExecutorBin = "/usr/libexec/selenite-controller/podman-volume-helper"
)

type volumeManager struct {
	runner executil.Runner
}

func New() *volumeManager {
	return &volumeManager{
		runner: executil.ExecRunner{},
	}
}

func (m *volumeManager) Export(ctx context.Context, name string, w io.Writer) error {
	enc, err := zstd.NewWriter(w)
	if err != nil {
		return fmt.Errorf("zstd encoder init: %w", err)
	}

	defer func() {
		if cerr := enc.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("zstd close: %w", cerr)
		}
	}()

	if err := m.runner.Run(
		ctx,
		ExecutorBin,
		[]string{"export", name},
		executil.WithStdout(enc),
	); err != nil {
		return fmt.Errorf("volume export: %w", err)
	}

	return nil
}

func (m *volumeManager) Import(ctx context.Context, name string, r io.Reader) error {
	dec, err := zstd.NewReader(r)
	if err != nil {
		return fmt.Errorf("zstd decoder init: %w", err)
	}

	defer func() {
		dec.Close()
	}()

	if err := m.runner.Run(
		ctx,
		ExecutorBin,
		[]string{"import", name, "-"},
		executil.WithStdin(dec),
	); err != nil {
		return fmt.Errorf("volume import: %w", err)
	}

	return nil
}

func (m *volumeManager) Prune(ctx context.Context, w io.Writer) error {
	if err := m.runner.Run(
		ctx,
		ExecutorBin,
		[]string{"prune", "-f"},
		executil.WithStdout(w),
	); err != nil {
		return fmt.Errorf("volume import: %w", err)
	}

	return nil
}
