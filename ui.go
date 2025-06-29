package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

const (
	viewportWidth    = 30
	colNameWidth     = 15
	colVersionWidth  = 15
	colTapWidth      = 15
	colDescWidthMin  = 30
	colInstallsWidth = 8
	colStatusWidth   = 15

	outputMaxLines = 10
)

// --- Styles ---

var (
	headerColor             = lipgloss.Color("#FFD580")
	highlightColor          = headerColor
	highlightForegroudColor = lipgloss.Color("#2E2E2E")
	borderColor             = lipgloss.Color("240")
	installedColor          = lipgloss.Color("#22C55E")
	uninstalledColor        = lipgloss.Color("#EF4444")
	pinnedColor             = lipgloss.Color("#B57EDC")

	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(borderColor)

	installedStyle = lipgloss.NewStyle().
			Foreground(installedColor)

	uninstalledStyle = lipgloss.NewStyle().
				Foreground(uninstalledColor)

	outdatedStyle = lipgloss.NewStyle().
			Foreground(highlightColor)

	pinnedStyle = lipgloss.NewStyle().
			Foreground(pinnedColor)

	headerStyle = lipgloss.NewStyle().
			Foreground(headerColor).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Padding(1 /* top */, 2 /* horizontal */, 0 /* bottom */)
	keyStyle = lipgloss.NewStyle().
			Foreground(highlightColor)

	searchStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Margin(1 /* top */, 0 /* horizontal */, 0 /* bottom */)

	outputStyle = baseStyle.Copy()

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
		m.spinner.Style = spinnerStyle
		return fmt.Sprintf("\n %s Loading package data...\n\n", m.spinner.View())
	}

	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		baseStyle.Render(m.table.View()),
		baseStyle.Render(m.viewport.View()),
	)

	views := []string{
		searchStyle.Render(m.search.View()),
		mainContent,
	}
	if output := m.renderOutput(); output != "" {
		views = append(views, output)
	}
	views = append(views, m.renderHelp())

	return lipgloss.JoinVertical(lipgloss.Left, views...)
}

func (m *model) renderOutput() string {
	if !m.isExecuting {
		return ""
	}

	var output string
	if len(m.output) > outputMaxLines {
		output = strings.Join(m.output[len(m.output)-outputMaxLines:], "\n")
	} else {
		output = strings.Join(m.output, "\n")
	}
	return outputStyle.Render(output)
}

func (m *model) renderHelp() string {
	var b strings.Builder

	b.WriteString("General   : ")
	b.WriteString(keyStyle.Render("q"))
	b.WriteString(": quit ")
	b.WriteString(keyStyle.Render("r"))
	b.WriteString(": refresh ")
	b.WriteString(keyStyle.Render("/"))
	b.WriteString(": search ")
	b.WriteString(keyStyle.Render("Esc"))
	b.WriteString(": clear search ")
	b.WriteString(keyStyle.Render("Enter"))
	b.WriteString(": exit focus ")
	b.WriteString(keyStyle.Render("s"))
	b.WriteString(": sort by name/popularity")
	b.WriteString("\n")
	b.WriteString("Navigation: ")
	b.WriteString(keyStyle.Render("j/↓"))
	b.WriteString(": cursor down ")
	b.WriteString(keyStyle.Render("k/↑"))
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
	b.WriteString("Filters   : ")
	b.WriteString(keyStyle.Render("a"))
	b.WriteString(": all ")
	b.WriteString(keyStyle.Render("f"))
	b.WriteString(": formulae ")
	b.WriteString(keyStyle.Render("c"))
	b.WriteString(": casks ")
	b.WriteString(keyStyle.Render("i"))
	b.WriteString(": installed ")
	b.WriteString(keyStyle.Render("o"))
	b.WriteString(": outdated ")
	b.WriteString(keyStyle.Render("e"))
	b.WriteString(": explicitly installed")
	b.WriteString("\n")
	b.WriteString("Commands  : ")
	b.WriteString(keyStyle.Render("U"))
	b.WriteString(": upgrade all ")
	b.WriteString(keyStyle.Render("u"))
	b.WriteString(": upgrade selected ")
	b.WriteString(keyStyle.Render("t"))
	b.WriteString(": install selected ")
	b.WriteString(keyStyle.Render("x"))
	b.WriteString(": uninstall selected ")
	b.WriteString(keyStyle.Render("p"))
	b.WriteString(": pin selected ")
	b.WriteString(keyStyle.Render("P"))
	b.WriteString(": unpin selected")

	return helpStyle.Render(b.String())
}

// updateLayout recalculates component dimensions based on window size.
func (m *model) updateLayout() {
	// 2, 4, 6 are used to account for border, margin and prompt width (search box only)
	outputStyle = outputStyle.Copy().Width(m.width - 2)
	helpStyle = helpStyle.Copy().Width(m.width - 2)
	m.search.Width = m.width - 6
	tableWidth := m.width - viewportWidth - 4

	m.viewport.Width = viewportWidth
	m.table.SetWidth(tableWidth)

	searchHeight := lipgloss.Height(searchStyle.Render(m.search.View()))
	helpHeight := lipgloss.Height(m.renderHelp())
	outputHeight := lipgloss.Height(m.renderOutput())
	mainHeight := m.height - helpHeight - searchHeight - outputHeight - 4

	m.table.SetHeight(mainHeight)
	m.viewport.Height = mainHeight

	// Dynamically adjust the width of the description column.
	otherColsWidth := colNameWidth + colVersionWidth + colTapWidth + colInstallsWidth + colStatusWidth
	descWidth := tableWidth - otherColsWidth - 12 // 12 is purely a magic number, not sure why
	if descWidth < colDescWidthMin {
		descWidth = colDescWidthMin
	}
	m.table.SetColumns([]table.Column{
		{Title: "Name", Width: colNameWidth},
		{Title: "Version", Width: colVersionWidth},
		{Title: "Tap", Width: colTapWidth},
		{Title: "Description", Width: descWidth},
		{Title: "Installs", Width: colInstallsWidth},
		{Title: "Status", Width: colStatusWidth},
	})

	tableStyle := table.DefaultStyles()
	tableStyle.Header = tableStyle.Header.
		Foreground(headerColor).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderBottom(true).
		Bold(true)
	tableStyle.Selected = tableStyle.Selected.
		Foreground(highlightForegroudColor).
		Background(highlightColor).
		Bold(true)
	m.table.SetStyles(tableStyle)
}

// updateTable populates the table with the current viewPackages.
func (m *model) updateTable() {
	rows := make([]table.Row, len(m.viewPackages))
	for i, pkg := range m.viewPackages {
		version := pkg.Version
		if pkg.IsPinned {
			version = fmt.Sprintf("%s (Pin)", pkg.InstalledVersion)
		} else if pkg.IsOutdated {
			version = fmt.Sprintf("%s (New)", pkg.Version)
		}
		rows[i] = table.Row{
			pkg.Name,
			version,
			pkg.Tap,
			pkg.Desc,
			fmt.Sprintf("%*d", colInstallsWidth, pkg.InstallCount90d),
			pkg.Status,
		}
	}
	m.table.SetRows(rows)

	// Reset cursor if it's out of bounds
	if m.table.Cursor() >= len(rows) {
		m.table.SetCursor(0)
	}

	m.updateViewport()
}

// updateViewport sets the content of the details panel based on the selected package.
func (m *model) updateViewport() {
	if len(m.viewPackages) == 0 {
		m.viewport.SetContent("No packages match the current filter.")
		return
	}

	// Ensure selected index is valid
	selectedIndex := m.table.Cursor()
	if selectedIndex < 0 || selectedIndex >= len(m.viewPackages) {
		selectedIndex = 0
	}

	pkg := m.viewPackages[selectedIndex]

	version := pkg.Version
	if pkg.IsOutdated {
		version = fmt.Sprintf("%s -> %s", pkg.InstalledVersion, pkg.Version)
	} else if pkg.IsPinned {
		version = fmt.Sprintf("%s (Pin)", pkg.InstalledVersion)
	}

	status := pkg.Status
	statusSymbol := ""
	if pkg.IsPinned {
		statusSymbol = pinnedStyle.Render("")
	} else if pkg.IsOutdated {
		statusSymbol = outdatedStyle.Render("")
	} else if pkg.IsInstalled {
		statusSymbol = installedStyle.Render("✓")
	} else {
		statusSymbol = uninstalledStyle.Render("✗")
	}
	status = fmt.Sprintf("%s %s\n", statusSymbol, status)

	var b strings.Builder
	b.WriteString(headerStyle.Render(pkg.Name))
	b.WriteString(fmt.Sprintf("\n%s\n\n", pkg.Desc))
	b.WriteString(fmt.Sprintf("Version: %s\n", version))
	b.WriteString(fmt.Sprintf("Tap: %s\n", pkg.Tap))
	b.WriteString(fmt.Sprintf("Homepage: %s\n", pkg.Homepage))
	b.WriteString(fmt.Sprintf("License: %s\n\n", pkg.License))
	b.WriteString(fmt.Sprintf("Status: %s\n", status))
	b.WriteString(fmt.Sprintf("90-Day Installs: %d\n\n", pkg.InstallCount90d))

	b.WriteString("Dependencies:\n")
	if len(pkg.Dependencies) > 0 {
		for _, dep := range pkg.Dependencies {
			depPkg := m.getPackage(dep)
			if depPkg != nil && depPkg.IsInstalled {
				b.WriteString(fmt.Sprintf("  %s %s\n", installedStyle.Render("✓"), dep))
			} else {
				b.WriteString(fmt.Sprintf("  %s %s\n", uninstalledStyle.Render("✗"), dep))
				// For uninstall dependencies, show all recursive uninstalled dependencies
				recursiveMissingDeps := m.getRecursiveMissingDeps(dep)
				for _, d := range recursiveMissingDeps {
					if p := m.getPackage(d); !p.IsInstalled {
						b.WriteString(fmt.Sprintf("    %s %s\n", uninstalledStyle.Render("✗"), d))
					}
				}
			}
		}
	} else {
		b.WriteString("  None\n")
	}

	if pkg.IsInstalled && len(pkg.Dependents) > 0 {
		b.WriteString("\nRequired By (installed):\n")
		for _, dep := range pkg.Dependents {
			b.WriteString(fmt.Sprintf("  %s %s\n", installedStyle.Render("✓"), dep))
		}
	}

	vpStyle := lipgloss.NewStyle().Width(viewportWidth)
	m.viewport.SetContent(vpStyle.Render(b.String()))
	m.viewport.GotoTop() // Reset scroll position
}

func (m *model) getRecursiveMissingDeps(pkgName string) []string {
	pkg := m.getPackage(pkgName)
	if pkg.IsInstalled {
		return []string{}
	} else {
		deps := pkg.Dependencies
		depsCopy := append([]string{}, deps...)
		for _, dep := range depsCopy {
			deps = append(deps, m.getRecursiveMissingDeps(dep)...)
		}
		return deps
	}
}
