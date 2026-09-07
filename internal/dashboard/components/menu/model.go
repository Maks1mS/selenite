// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package menu

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Item struct {
	Label string
}

type SelectMsg struct {
	Index int
	Item  Item
}

var keyOrder = []string{"f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10"}

type Styles struct {
	Bar    lipgloss.Style
	Key    lipgloss.Style
	Label  lipgloss.Style
	Active lipgloss.Style
}

func DefaultStyles() Styles {
	return Styles{
		Active: lipgloss.NewStyle().
			Foreground(lipgloss.Black).
			Background(lipgloss.White),
	}
}

type Model struct {
	items  []Item
	active int
	width  int
	styles Styles
}

func New(items []Item) *Model {
	return &Model{
		items:  items,
		active: 0,
		styles: DefaultStyles(),
	}
}

func (m *Model) SetWidth(w int)     { m.width = w }
func (m *Model) SetStyles(s Styles) { m.styles = s }
func (m *Model) SetActive(i int)    { m.active = i }
func (m *Model) Active() int        { return m.active }

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width

	case tea.KeyPressMsg:
		key := msg.String()
		for i, k := range keyOrder {
			if i >= len(m.items) {
				break
			}
			if key == k {
				m.active = i
				item := m.items[i]
				return m, func() tea.Msg {
					return SelectMsg{Index: i, Item: item}
				}
			}
		}
	}
	return m, nil
}

func (m *Model) View() tea.View {
	return tea.NewView(m.render())
}

func (m *Model) render() string {
	var b strings.Builder
	for i, item := range m.items {
		keyPart := m.styles.Key.Render(fmt.Sprintf("[F%d]", i+1))

		labelStyle := m.styles.Label
		if i == m.active {
			labelStyle = m.styles.Active
		}
		labelPart := labelStyle.Render(item.Label)

		if i != 0 {
			b.WriteString(" | ")
		}
		b.WriteString(keyPart)
		b.WriteString(" ")
		b.WriteString(labelPart)
	}

	bar := b.String()
	if m.width > 0 {
		return m.styles.Bar.Width(m.width).Render(bar)
	}
	return m.styles.Bar.Render(bar)
}
