// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package network

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/goccy/go-yaml"

	"altlinux.space/alt-atomic/selenite/internal/dashboard/pages/base"
	"altlinux.space/alt-atomic/selenite/internal/nmstate"
)

const (
	kindDHCP   = "dhcp"
	kindStatic = "static"

	interfaceNone = "(none)"
)

type page struct {
	backend Backend

	width, height int
	err           error

	ifaces        map[string]nmstate.Interface
	currentConfig string

	formData FormData

	form *huh.Form
}

func (m *page) Init() tea.Cmd {
	return m.form.Init()
}

func (m *page) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case base.ActivatedMsg:
		m.reload()
		return m, m.form.GetFocusedField().Focus()
	}

	if m.form.State == huh.StateCompleted {
		var applyErr error
		if m.formData.Save {
			applyErr = m.applyConfig()
		}
		m.reload(applyErr)
		return m, tea.Batch(m.form.Init(), func() tea.Msg { return base.NetworkUpdateMsg{} })
	}

	return m, cmd
}

// reload rebuilds the form and the current config, keeping errs for display.
func (m *page) reload(errs ...error) {
	var err error
	m.ifaces, err = m.backend.ListInterfaces()
	errs = append(errs, err)
	m.form = newNetworkForm(m)
	m.currentConfig, err = m.backend.CurrentConfig()
	errs = append(errs, err)
	m.err = errors.Join(errs...)
}

func (m *page) applyConfig() error {
	config, err := m.buildConfig()
	if err != nil {
		return err
	}
	if err := m.backend.ApplyConfig(config); err != nil {
		return fmt.Errorf("apply network config: %w", err)
	}
	return nil
}

func (m *page) buildConfig() (nmstate.SeleniteConfig, error) {
	return m.formData.ToSeleniteConfig(m.ifaces)
}

func (m *page) buildConfigString() (string, error) {
	cfg, err := m.buildConfig()
	if err != nil {
		return "", err
	}

	b, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("yaml.Marshal: %w", err)
	}

	return string(b), err
}

func (m *page) View() tea.View {
	newConfig, err := m.buildConfigString()
	if err != nil {
		newConfig = lipgloss.NewStyle().
			Foreground(lipgloss.Red).
			Render(err.Error())
	}

	lipgloss.NewStyle().
		AlignHorizontal(lipgloss.Center).
		Width(m.width / 3).
		Render("Current config")

	borderCols := 2
	budget := m.width - borderCols - 2

	base := budget / 3
	rem := budget % 3

	widths := [3]int{base, base, base}
	for i := range rem {
		widths[i]++
	}

	leftWidth := widths[0]
	midWidth := widths[1]
	rightWidth := widths[2]

	inner := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().
			Width(leftWidth).
			Height(m.height-1).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderRight(true).
			Render(m.form.View()),
		lipgloss.JoinVertical(
			lipgloss.Top,
			lipgloss.NewStyle().
				AlignHorizontal(lipgloss.Center).
				Width(midWidth).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				Render("Current config"),
			lipgloss.NewStyle().
				Width(midWidth).
				Height(m.height-3).
				Render(m.currentConfigView(midWidth)),
		),
		lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderLeft(true).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Top,
					lipgloss.NewStyle().
						AlignHorizontal(lipgloss.Center).
						Width(rightWidth).
						BorderStyle(lipgloss.NormalBorder()).
						BorderBottom(true).
						Render("New config"),
					lipgloss.NewStyle().
						Width(rightWidth).
						Height(m.height-3).
						Render(newConfig),
				),
			),
	)

	v := tea.NewView(
		lipgloss.NewStyle().
			Width(m.width).
			Height(m.height + 1).
			MaxHeight(m.height + 1).
			Border(lipgloss.RoundedBorder()).
			Render(inner),
	)
	return v
}

func (m *page) currentConfigView(width int) string {
	if m.err == nil {
		return m.currentConfig
	}
	errView := lipgloss.NewStyle().
		Width(width).
		Foreground(lipgloss.Red).
		Render(m.err.Error())
	return lipgloss.JoinVertical(lipgloss.Left, errView, "", m.currentConfig)
}

func New(backend Backend) *page {
	p := &page{backend: backend}
	p.ifaces, p.err = backend.ListInterfaces()
	p.form = newNetworkForm(p)
	p.formData.InterfaceName = interfaceNone

	return p
}

func newNetworkForm(m *page) *huh.Form {
	ifacenames := make([]string, 0, len(m.ifaces)+1)
	ifacenames = append(ifacenames, interfaceNone)
	ifacenames = append(ifacenames, slices.Sorted(maps.Keys(m.ifaces))...)

	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Hostname").Value(&m.formData.Hostname),
			huh.NewInput().Title("DNS Servers").Value(&m.formData.Dns),
			huh.NewSelect[string]().Title("Interface").
				Options(huh.NewOptions(ifacenames...)...).
				Value(&m.formData.InterfaceName),
		),
		huh.NewGroup(
			huh.NewSelect[string]().Title("Address mode").
				Options(huh.NewOption("DHCP", kindDHCP), huh.NewOption("Static", kindStatic)).
				Value(&m.formData.Kind),
		).WithHideFunc(func() bool { return m.formData.InterfaceName == interfaceNone }),
		huh.NewGroup(
			huh.NewInput().Title("Addresses").Value(&m.formData.Address),
			huh.NewInput().Title("Gateway").Value(&m.formData.Gateway),
		).WithHideFunc(func() bool { return m.formData.InterfaceName == interfaceNone || m.formData.Kind != kindStatic }),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Apply configuration?").
				Affirmative("Save").
				Negative("Cancel").
				Value(&m.formData.Save),
		),
	).
		WithTheme(huh.ThemeFunc(huh.ThemeBase)).
		WithLayout(huh.LayoutStack).
		WithShowHelp(false)
}
