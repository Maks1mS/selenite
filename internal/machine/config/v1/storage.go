// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"altlinux.space/alt-atomic/selenite/internal/executil"
	"altlinux.space/alt-atomic/selenite/internal/helpers/root/storage"
)

const storageHelperBin = "/usr/libexec/selenite-controller/storage-helper"

// StorageApplier applies host-level storage mounts.
type StorageApplier struct {
	newParams  StorageConfig
	prevParams StorageConfig
	r          executil.Runner
}

func NewStorageApplier(newParams, prevParams StorageConfig, r executil.Runner) *StorageApplier {
	return &StorageApplier{newParams: newParams, prevParams: prevParams, r: r}
}

func (a *StorageApplier) Name() string {
	return "storage"
}

// Validate checks the mount set against the whitelist in
// internal/helpers/root/storage (which the setuid helper re-validates).
func (a *StorageApplier) Validate(_ context.Context) error {
	//nolint:wrapcheck // validation errors are self-describing; the orchestrator adds "validate:"
	return storage.Validate(storageMounts(a.newParams.Mounts), "storage.mounts")
}

func (a *StorageApplier) Apply(ctx context.Context) error {
	if reflect.DeepEqual(a.newParams, a.prevParams) {
		return nil
	}
	return a.runHelper(ctx, a.newParams)
}

func (a *StorageApplier) Rollback(ctx context.Context) error {
	return a.runHelper(ctx, a.prevParams)
}

func (a *StorageApplier) runHelper(ctx context.Context, p StorageConfig) error {
	b, err := json.Marshal(storageMounts(p.Mounts))
	if err != nil {
		return fmt.Errorf("marshal mounts: %w", err)
	}
	var stderr bytes.Buffer
	if err := a.r.Run(
		ctx,
		storageHelperBin,
		[]string{"apply"},
		executil.WithStdin(bytes.NewReader(b)),
		executil.WithStderr(&stderr),
	); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String())
	}
	return nil
}

// storageMounts converts config mounts into the storage-helper wire format.
func storageMounts(mounts []MountSpec) []storage.Mount {
	out := make([]storage.Mount, 0, len(mounts))
	for _, m := range mounts {
		out = append(out, storage.Mount{
			Type:    m.Type,
			Source:  m.Source,
			Target:  m.Target,
			Fs:      m.Fs,
			Options: m.Options.OrEmpty(),
		})
	}
	return out
}
