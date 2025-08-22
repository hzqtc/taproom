package model

import (
	"fmt"
	"strings"
	"taproom/internal/ui"
	"taproom/internal/util"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/pflag"
)

const (
	sidePanelWidthMin = 30
	outputMaxLines    = 10
)

var (
	flagHideHelp = pflag.Bool("hide-help", false, "Hide the help text")
)

// --- Styles ---

var (
	highlightColor  = lipgloss.Color("#FFD580")
	borderColor     = lipgloss.Color("#909090")
	errBorderColor  = deprecatedColor
	deprecatedColor = lipgloss.Color("#EF4444")

	roundedBorder = lipgloss.RoundedBorder()

	baseStyle = lipgloss.NewStyle().
			BorderStyle(roundedBorder).
			BorderForeground(borderColor)

	headerStyle = lipgloss.NewStyle().
			Foreground(highlightColor).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Padding(1 /* top */, 2 /* horizontal */, 0 /* bottom */)

	keyStyle = lipgloss.NewStyle().
			Foreground(highlightColor)

	outputStyle = baseStyle.
			Margin(1 /* top */, 0 /* horizontal */, 0 /* bottom */).
			Padding(0, 1)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(highlightColor)
)

// --- View & Layout ---

// View is the main render function for the application.
func (m model) View() string {
	if m.errorMsg != "" {
		return fmt.Sprintf("An error occurred: %s\nPress 'q' to quit.", m.errorMsg)
	}

	if m.isLoading {
		var b strings.Builder
		m.spinner.Style = spinnerStyle
		b.WriteString(
			fmt.Sprintf(
				"%s\n%s\n\n%s Loading...",
				headerStyle.Render(logo),
				m.loadingPrgs.Progress(headerStyle.Render("Done")),
				m.spinner.View(),
			),
		)
		if m.loadTimer {
			b.WriteString(m.stopwatch.View())
		}
		return b.String()
	}

	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.table.View(),
		m.detailPanel.View(),
	)

	topContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.search.View(),
		m.filterView.View(),
	)

	views := []string{
		topContent,
		mainContent,
		m.renderStats(),
	}
	if output := m.renderOutput(); output != "" {
		views = append(views, output)
	}
	if !*flagHideHelp {
		views = append(views, m.renderHelp())
	}

	return lipgloss.JoinVertical(lipgloss.Left, views...)
}

func (m *model) renderOutput() string {
	if len(m.output) == 0 {
		return ""
	}

	var output string
	if len(m.output) > outputMaxLines {
		output = strings.Join(m.output[len(m.output)-outputMaxLines:], "\n")
	} else {
		output = strings.Join(m.output, "\n")
	}

	if m.commandErr {
		return outputStyle.BorderForeground(errBorderColor).Render(output)
	} else {
		return outputStyle.Render(output)
	}
}

func renderKey(k key.Binding) string {
	return keyStyle.Render(k.Keys()[0])
}

func (m *model) renderHelp() string {
	var b strings.Builder

	b.WriteString("General   : ")
	b.WriteString(renderKey(m.keys.Quit))
	b.WriteString(": quit ")
	b.WriteString(renderKey(m.keys.Refresh))
	b.WriteString(": refresh ")
	b.WriteString(renderKey(m.keys.SwitchFocus))
	b.WriteString(": switch focus ")
	b.WriteString(renderKey(m.keys.FocusSearch))
	b.WriteString(": search ")
	b.WriteString(renderKey(m.keys.Esc))
	b.WriteString(": clear search ")
	b.WriteString(renderKey(m.keys.Enter))
	b.WriteString(": exit search ")
	b.WriteString(keyStyle.Render("s") + "/" + keyStyle.Render("S"))
	b.WriteString(": sorting")
	b.WriteString("\n")
	b.WriteString("Navigation: ")
	b.WriteString(keyStyle.Render("j") + "/" + keyStyle.Render("↓"))
	b.WriteString(": cursor down ")
	b.WriteString(keyStyle.Render("k") + "/" + keyStyle.Render("↑"))
	b.WriteString(": cursor up ")
	b.WriteString(keyStyle.Render("PageUp"))
	b.WriteString(": prev page ")
	b.WriteString(keyStyle.Render("PageDown"))
	b.WriteString(": next page ")
	b.WriteString(keyStyle.Render("g"))
	b.WriteString(": go to top ")
	b.WriteString(keyStyle.Render("G"))
	b.WriteString(": go to bottom")
	b.WriteString("\n")
	b.WriteString("Filter    : ")
	b.WriteString(keyStyle.Render("a"))
	b.WriteString(": all (no filter) ")
	b.WriteString(keyStyle.Render("f"))
	b.WriteString(": formulae ")
	b.WriteString(keyStyle.Render("c"))
	b.WriteString(": casks ")
	b.WriteString(keyStyle.Render("i"))
	b.WriteString(": installed ")
	b.WriteString(keyStyle.Render("o"))
	b.WriteString(": outdated ")
	b.WriteString(keyStyle.Render("e"))
	b.WriteString(": explicitly installed ")
	b.WriteString(keyStyle.Render("v"))
	b.WriteString(": active")
	b.WriteString("\n")
	b.WriteString("Commands  : ")
	b.WriteString(renderKey(m.keys.OpenHomePage))
	b.WriteString(": home page ")
	b.WriteString(renderKey(m.keys.OpenBrewUrl))
	b.WriteString(": brew.sh ")
	b.WriteString(renderKey(m.keys.OpenRelease))
	b.WriteString(": release page ")
	b.WriteString(renderKey(m.keys.UpgradeAll))
	b.WriteString(": upgrade all ")
	b.WriteString(renderKey(m.keys.Upgrade))
	b.WriteString(": upgrade ")
	b.WriteString(renderKey(m.keys.Install))
	b.WriteString(": install ")
	b.WriteString(renderKey(m.keys.Remove))
	b.WriteString(": uninstall ")
	b.WriteString(renderKey(m.keys.Pin))
	b.WriteString(": pin ")
	b.WriteString(renderKey(m.keys.Unpin))
	b.WriteString(": unpin ")
	b.WriteString(renderKey(m.keys.CleanUp))
	b.WriteString(": cleanup")

	return helpStyle.Render(b.String())
}

func (m *model) updateFocusBorder() {
	switch m.focusMode {
	case focusSearch:
		m.search.SetFocused(true)
		m.table.SetFocused(false)
		m.detailPanel.SetFocused(false)
	case focusTable:
		m.search.SetFocused(false)
		m.table.SetFocused(true)
		m.detailPanel.SetFocused(false)
	case focusDetail:
		m.search.SetFocused(false)
		m.table.SetFocused(false)
		m.detailPanel.SetFocused(true)
	}
}

// updateLayout recalculates component dimensions based on window size.
func (m *model) updateLayout() {
	m.updateFocusBorder()

	// 2, 4, 6, 8 are used to account for border, margin and prompt width (search box only)
	outputStyle = outputStyle.Width(m.width - 2)
	helpStyle = helpStyle.Width(m.width - 2)

	sidePanelWidth := max(sidePanelWidthMin, m.width-ui.MaxTableWidth-4)
	tableWidth := m.width - sidePanelWidth - 4

	mainHeight := m.height - 4
	mainHeight -= lipgloss.Height(m.search.View())
	mainHeight -= lipgloss.Height(m.renderStats())
	if !*flagHideHelp {
		mainHeight -= lipgloss.Height(m.renderHelp())
	}
	mainHeight -= lipgloss.Height(m.renderOutput())

	m.filterView.SetWidth(sidePanelWidth)
	m.search.SetWidth(m.width - sidePanelWidth - 8)
	m.table.SetDimensions(tableWidth, mainHeight)
	m.detailPanel.SetDimension(sidePanelWidth-2, mainHeight)
}

func (m *model) renderStats() string {
	var formulaeNum, casksNum int
	var installedFormulaeNum, installedFormulaeDepNum, installedCasksNum int
	var formulaeSize, casksSize int64
	for _, pkg := range m.table.Packages() {
		if pkg.IsCask {
			casksNum++
		} else {
			formulaeNum++
		}

		if !pkg.IsInstalled {
			continue
		}

		if pkg.IsCask {
			installedCasksNum++
			casksSize += pkg.Size
		} else {
			installedFormulaeNum++
			formulaeSize += pkg.Size
			if pkg.InstalledAsDependency {
				installedFormulaeDepNum++
			}
		}
	}
	return helpStyle.Render(
		fmt.Sprintf(
			"%s Formulae available | %s Casks available | %s Formulae (incl. %s deps) installed taking %s | %s Casks installed taking %s",
			keyStyle.Render(fmt.Sprintf("%d", formulaeNum)),
			keyStyle.Render(fmt.Sprintf("%d", casksNum)),
			keyStyle.Render(fmt.Sprintf("%d", installedFormulaeNum)),
			keyStyle.Render(fmt.Sprintf("%d", installedFormulaeDepNum)),
			keyStyle.Render(util.FormatSize(formulaeSize)),
			keyStyle.Render(fmt.Sprintf("%d", installedCasksNum)),
			keyStyle.Render(util.FormatSize(casksSize)),
		))
}
