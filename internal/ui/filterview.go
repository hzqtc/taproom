package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/pflag"
)

type FilterChangedMsg struct {
	filters []Filter
}

type FilterViewModel struct {
	fg    filterGroup
	width int

	filterAll       key.Binding
	filterFormulae  key.Binding
	filterCasks     key.Binding
	filterInstalled key.Binding
	filterOutdated  key.Binding
	filterExplicit  key.Binding
	filterActive    key.Binding
}

var flagFilters = pflag.StringSliceP(
	"filters",
	"f",
	[]string{},
	"Filters to enable (comma separated no space).\n"+
		"Pick 0 or 1 filter from each group: (Formulae, Casks), (Installed, Outdated, Expl. Installed, Active)",
)

var filterStyle = baseStyle.
	Padding(0, 1).
	Margin(1, 0)

func NewFilterViewModel() FilterViewModel {
	fg, err := parseFilterGroup(*flagFilters)
	if err != nil {
		fg = emptyFilterGroup
	}

	return FilterViewModel{
		fg:              fg,
		filterAll:       key.NewBinding(key.WithKeys("a")),
		filterFormulae:  key.NewBinding(key.WithKeys("f")),
		filterCasks:     key.NewBinding(key.WithKeys("c")),
		filterInstalled: key.NewBinding(key.WithKeys("i")),
		filterOutdated:  key.NewBinding(key.WithKeys("o")),
		filterExplicit:  key.NewBinding(key.WithKeys("e")),
		filterActive:    key.NewBinding(key.WithKeys("v")),
	}
}

func (m FilterViewModel) Update(msg tea.Msg) (FilterViewModel, tea.Cmd) {
	prevFg := m.fg
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.filterAll):
			m.fg.reset()
		case key.Matches(msg, m.filterFormulae):
			m.fg.toggleFilter(FilterFormulae)
		case key.Matches(msg, m.filterCasks):
			m.fg.toggleFilter(FilterCasks)
		case key.Matches(msg, m.filterInstalled):
			m.fg.toggleFilter(FilterInstalled)
		case key.Matches(msg, m.filterOutdated):
			m.fg.toggleFilter(FilterOutdated)
		case key.Matches(msg, m.filterExplicit):
			m.fg.toggleFilter(FilterExplicitlyInstalled)
		case key.Matches(msg, m.filterActive):
			m.fg.toggleFilter(FilterActive)
		}
	}

	if prevFg != m.fg {
		return m, m.sendFilterChangedMsg()
	} else {
		return m, nil
	}
}

func (m *FilterViewModel) sendFilterChangedMsg() tea.Cmd {
	return func() tea.Msg {
		return FilterChangedMsg{
			filters: m.Value(),
		}
	}
}

func (m FilterViewModel) View() string {
	return filterStyle.Render(m.fg.String())
}

func (m *FilterViewModel) SetWidth(w int) {
	filterStyle = filterStyle.
		BorderStyle(getRoundedBorderWithTitle("Filters", w)).
		Width(w)
}

func (m *FilterViewModel) Value() []Filter {
	return m.fg.split()
}
