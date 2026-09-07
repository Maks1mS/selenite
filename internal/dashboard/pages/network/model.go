// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package network

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-yaml"
	"go.alt-gnome.ru/x/runner"

	"altlinux.space/alt-atomic/selenite/internal/dashboard/pages/base"
)

const (
	kindDHCP   = "dhcp"
	kindStatic = "static"

	interfaceNone = "(none)"
)

type FormData struct {
	Hostname      string
	Dns           string
	InterfaceName string
	Kind          string
	Address       string
	Gateway       string
	Save          bool
}

type FormDataNormalized struct {
	Hostname      string   `validate:"omitempty,hostname"`
	Dns           []string `validate:"dive,ip"`
	InterfaceName string
	Kind          string
	Address       []string `validate:"dive,cidr"`
	Gateway       string   `validate:"required_unless=Kind dhcp,omitempty,ip"`
}

type page struct {
	width, height int
	err           error

	ifaces        map[string]Interface
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
		m.form = newNetworkForm(m)
		m.currentConfig, m.err = m.getConfig()
		return m, m.form.GetFocusedField().Focus()
	}

	if m.form.State == huh.StateCompleted {
		if m.formData.Save {
			if err := m.applyConfig(); err != nil {
				m.err = err
			}
			m.currentConfig, m.err = m.getConfig()
		}
		m.form = newNetworkForm(m)
		return m, tea.Batch(m.form.Init(), func() tea.Msg { return base.NetworkUpdateMsg{} })
	}

	return m, cmd
}

func (m *page) getConfig() (string, error) {
	r := runner.NewLocal()
	var stdout, stderr bytes.Buffer
	err := r.Run(
		context.Background(),
		"/usr/libexec/selenite-controller/selenite-dashboard-helper",
		[]string{"_dashboard-get-network"},
		runner.WithStdout(&stdout),
		runner.WithStderr(&stderr),
	)
	if err != nil {
		if stderr.Len() != 0 {
			return "", fmt.Errorf("nmstatectl apply: %s: %w", strings.TrimSpace(stderr.String()), err)
		}
		return "", fmt.Errorf("nmstatectl apply: %w", err)
	}

	return stdout.String(), nil
}

func (m *page) applyConfig() error {
	config, err := m.buildConfig()
	if err != nil {
		return err
	}

	b, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("yaml.Marshal: %w", err)
	}

	r := runner.NewLocal()
	var stderr bytes.Buffer
	err = r.Run(
		context.Background(),
		"/usr/libexec/selenite-controller/selenite-dashboard-helper",
		[]string{"_dashboard-set-network"},
		runner.WithStdin(bytes.NewBuffer(b)),
		runner.WithStderr(&stderr),
	)
	if err != nil {
		if stderr.Len() != 0 {
			return fmt.Errorf("nmstatectl apply: %s: %w", strings.TrimSpace(stderr.String()), err)
		}
		return fmt.Errorf("nmstatectl apply: %w", err)
	}

	return nil
}

func (m *page) buildConfig() (SeleniteConfig, error) {
	state := DesiredState{}

	input := FormDataNormalized{}
	input.Hostname = m.formData.Hostname
	input.Dns = strings.Fields(m.formData.Dns)
	input.InterfaceName = m.formData.InterfaceName
	input.Kind = m.formData.Kind
	input.Address = strings.Fields(m.formData.Address)
	input.Gateway = m.formData.Gateway

	validate := validator.New(validator.WithRequiredStructEnabled())
	err := validate.Struct(input)
	if err != nil {
		return SeleniteConfig{}, fmt.Errorf("validate error: %w", err)
	}

	if input.Hostname != "" {
		state.Hostname.Config = input.Hostname
	}
	if len(input.Dns) != 0 {
		state.DNSResolver.Config.Server = input.Dns
	}
	if input.InterfaceName != interfaceNone {
		iface := Interface{
			Name:  input.InterfaceName,
			Type:  m.ifaces[input.InterfaceName].Type,
			State: "up",
			Ipv4:  Ipv4{Enabled: true},
		}

		switch input.Kind {
		case kindDHCP:
			iface.Ipv4.Dhcp = true
		case kindStatic:
			iface.Ipv4.Address = []Address{}
			iface.Ipv6.Address = []Address{}

			for _, addr := range input.Address {
				ipAddr, ipNet, err := net.ParseCIDR(addr)
				if err != nil {
					continue
				}

				prefixLength, _ := ipNet.Mask.Size()

				address := Address{
					IP:           ipAddr.String(),
					PrefixLength: prefixLength,
				}

				if ipAddr.To4() != nil {
					iface.Ipv4.Address = append(iface.Ipv4.Address, address)
				} else {
					iface.Ipv6.Address = append(iface.Ipv6.Address, address)
				}
			}
			state.Routes.Config = []RoutesConfig{
				{
					Destination:      "0.0.0.0/0",
					NextHopAddress:   input.Gateway,
					NextHopInterface: input.InterfaceName,
				},
			}
		}

		state.Interfaces = []Interface{iface}
	}

	return SeleniteConfig{Network: NetworkConfig{state}}, nil
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
				Render(m.currentConfig),
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

func New() *page {
	p := &page{}
	p.form = newNetworkForm(p)
	p.formData.InterfaceName = interfaceNone

	return p
}

func newNetworkForm(m *page) *huh.Form {
	m.ifaces = m.listInterfaces()

	ifacenames := make([]string, 0, len(m.ifaces)+1)
	ifacenames = append(ifacenames, interfaceNone)
	for k := range m.ifaces {
		ifacenames = append(ifacenames, k)
	}

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

func (m *page) listInterfaces() map[string]Interface {
	r := runner.NewLocal()

	var b bytes.Buffer
	m.err = r.Run(context.Background(), "/usr/bin/nmstatectl", []string{"show"}, runner.WithStdout(&b))
	if m.err != nil {
		return nil
	}

	var s DesiredState
	m.err = yaml.Unmarshal(b.Bytes(), &s)
	if m.err != nil {
		return nil
	}

	data := map[string]Interface{}
	for _, i := range s.Interfaces {
		if slices.Contains([]string{"linux-bridge", "loopback", "tun", "veth"}, i.Type) {
			continue
		}
		data[i.Name] = i
	}

	return data
}
