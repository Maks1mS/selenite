// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package network

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"go.alt-gnome.ru/x/runner"

	"altlinux.space/alt-atomic/selenite/internal/nmstate"
)

const dashboardHelper = "/usr/libexec/selenite-controller/selenite-dashboard-helper"

// Backend reads and applies the machine network configuration.
type Backend interface {
	ListInterfaces() (map[string]nmstate.Interface, error)
	CurrentConfig() (string, error)
	ApplyConfig(config nmstate.SeleniteConfig) error
}

// SystemBackend talks to nmstate and selenite-controller on the local machine.
type SystemBackend struct{}

func (SystemBackend) ListInterfaces() (map[string]nmstate.Interface, error) {
	r := runner.NewLocal()

	var b bytes.Buffer
	if err := r.Run(context.Background(), "/usr/bin/nmstatectl", []string{"show"}, runner.WithStdout(&b)); err != nil {
		return nil, fmt.Errorf("list interfaces: %w", err)
	}

	var s nmstate.DesiredState
	if err := yaml.Unmarshal(b.Bytes(), &s); err != nil {
		return nil, fmt.Errorf("list interfaces: parse nmstatectl output: %w", err)
	}

	data := map[string]nmstate.Interface{}
	for _, i := range s.Interfaces {
		if slices.Contains([]string{"linux-bridge", "loopback", "tun", "veth"}, i.Type) {
			continue
		}
		data[i.Name] = i
	}

	return data, nil
}

func (SystemBackend) CurrentConfig() (string, error) {
	r := runner.NewLocal()
	var stdout, stderr bytes.Buffer
	err := r.Run(
		context.Background(),
		dashboardHelper,
		[]string{"_dashboard-get-network"},
		runner.WithStdout(&stdout),
		runner.WithStderr(&stderr),
	)
	if err != nil {
		if stderr.Len() != 0 {
			return "", fmt.Errorf("get network config: %s: %w", strings.TrimSpace(stderr.String()), err)
		}
		return "", fmt.Errorf("get network config: %w", err)
	}

	return stdout.String(), nil
}

func (SystemBackend) ApplyConfig(config nmstate.SeleniteConfig) error {
	b, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("yaml.Marshal: %w", err)
	}

	r := runner.NewLocal()
	var stderr bytes.Buffer
	err = r.Run(
		context.Background(),
		dashboardHelper,
		[]string{"_dashboard-set-network"},
		runner.WithStdin(bytes.NewBuffer(b)),
		runner.WithStderr(&stderr),
	)
	if err != nil {
		if stderr.Len() != 0 {
			return fmt.Errorf("%s: %w", strings.TrimSpace(stderr.String()), err)
		}
		return fmt.Errorf("run %s: %w", dashboardHelper, err)
	}

	return nil
}
