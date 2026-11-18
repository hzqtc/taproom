package ui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type SearchMsg struct {
	Query string
}

type SearchInputModel struct {
	input  textinput.Model
	cancel key.Binding
}

var searchStyle = baseStyle.
	Margin(1 /* top */, 0 /* horizontal */, 0 /* bottom */)

func NewSearchInputModel() SearchInputModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search packages..."
	searchInput.Prompt = " / "
	return SearchInputModel{
		input:  searchInput,
		cancel: key.NewBinding(key.WithKeys("esc")),
	}
}

func (m SearchInputModel) Update(msg tea.Msg) (SearchInputModel, tea.Cmd) {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, tea.Batch(cmd, m.sendSearchMsg())
}

func (m *SearchInputModel) Value() string {
	return m.input.Value()
}

func (m *SearchInputModel) Clear() tea.Cmd {
	m.input.SetValue("")
	return m.sendSearchMsg()
}

func (m *SearchInputModel) sendSearchMsg() tea.Cmd {
	return func() tea.Msg {
		return SearchMsg{
			Query: m.input.Value(),
		}
	}
}

func (m SearchInputModel) View() string {
	return searchStyle.Render(m.input.View())
}

func (m *SearchInputModel) SetWidth(w int) {
	m.input.Width = w
}

func (m *SearchInputModel) SetFocused(f bool) {
	if f {
		m.input.Focus()
		searchStyle = searchStyle.BorderForeground(focusedBorderColor)
	} else {
		m.input.Blur()
		searchStyle = searchStyle.BorderForeground(borderColor)
	}
}
