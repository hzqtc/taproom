package ui

import (
	"fmt"
	"strings"
	"taproom/internal/brew"
	"taproom/internal/data"
	"taproom/internal/util"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type DetailsPanelModel struct {
	pkg *data.Package
	vp  viewport.Model
}

var (
	installedColor   = lipgloss.Color("#22C55E")
	deprecatedColor  = lipgloss.Color("#EF4444")
	uninstalledColor = lipgloss.Color("#FBBF24")
	pinnedColor      = lipgloss.Color("#B57EDC")

	detailPanelStyle = baseStyle.
				Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Foreground(highlightColor).
			Bold(true)

	installedStyle = lipgloss.NewStyle().
			Foreground(installedColor)

	deprecatedStyle = lipgloss.NewStyle().
			Foreground(deprecatedColor)

	uninstalledStyle = lipgloss.NewStyle().
				Foreground(uninstalledColor)

	outdatedStyle = lipgloss.NewStyle().
			Foreground(highlightColor)

	pinnedStyle = lipgloss.NewStyle().
			Foreground(pinnedColor)
)

const (
	disabledSymbol    = ""
	deprecatedSymbol  = ""
	uninstalledSymbol = "✗"
	installedSymbol   = "✓"
	outdatedSymbol    = ""
	pinnedSymbol      = ""
)

func NewDetailsPanelModel() DetailsPanelModel {
	return DetailsPanelModel{}
}

func (m *DetailsPanelModel) SetDimension(width, height int) {
	m.vp.Width = width
	m.vp.Height = height
}

func (m *DetailsPanelModel) SetPackage(pkg *data.Package) {
	m.pkg = pkg
	m.updatePanel()
}

func (m *DetailsPanelModel) SetFocused(focused bool) {
	if focused {
		detailPanelStyle = detailPanelStyle.BorderForeground(focusedBorderColor)
	} else {
		detailPanelStyle = detailPanelStyle.BorderForeground(borderColor)
	}
}

func (m DetailsPanelModel) Update(msg tea.Msg) (DetailsPanelModel, tea.Cmd) {
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m DetailsPanelModel) View() string {
	detailPanelStyle = detailPanelStyle.
		BorderStyle(getRoundedBorderWithTitle("Details", m.vp.Width+2))
	return detailPanelStyle.Render(m.vp.View())
}

func getFormattedStatus(pkg *data.Package) string {
	var statusSymbol string
	if pkg.IsDisabled {
		statusSymbol = deprecatedStyle.Render(disabledSymbol)
	} else if pkg.IsDeprecated {
		statusSymbol = deprecatedStyle.Render(deprecatedSymbol)
	} else if pkg.IsPinned {
		statusSymbol = pinnedStyle.Render(pinnedSymbol)
	} else if pkg.IsOutdated {
		statusSymbol = outdatedStyle.Render(outdatedSymbol)
	} else if pkg.IsInstalled {
		statusSymbol = installedStyle.Render(installedSymbol)
	} else {
		statusSymbol = uninstalledStyle.Render(uninstalledSymbol)
	}
	return fmt.Sprintf("%s %s", statusSymbol, pkg.Status())
}

func (m *DetailsPanelModel) updatePanel() {
	if m.pkg == nil {
		m.vp.SetContent("No packages selected.")
		return
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render(fmt.Sprintf("%s %s", m.pkg.Symbol(), m.pkg.Name)))
	b.WriteString(fmt.Sprintf("\n%s\n\n", m.pkg.Desc))
	b.WriteString(fmt.Sprintf("Version: %s\n", m.pkg.LongVersion()))
	b.WriteString(fmt.Sprintf("Tap: %s\n", m.pkg.Tap))
	b.WriteString(fmt.Sprintf("Homepage: %s\n", m.pkg.Homepage))
	b.WriteString(fmt.Sprintf("License: %s\n", m.pkg.License))
	b.WriteString(fmt.Sprintf("Installs (90d): %d\n", m.pkg.InstallCount90d))

	b.WriteString(fmt.Sprintf("\nStatus: %s\n", getFormattedStatus(m.pkg)))
	if m.pkg.IsInstalled {
		b.WriteString(fmt.Sprintf("Size: %s\n", m.pkg.FormattedSize))
		b.WriteString(fmt.Sprintf("Installed on: %s\n", m.pkg.InstalledDate))
		if release := m.pkg.ReleaseInfo; release != nil {
			b.WriteString(fmt.Sprintf("Released on: %s\n", release.Date.Format(time.DateOnly)))
		}
	}

	if len(m.pkg.Conflicts) > 0 {
		b.WriteString("\nConflicts:\n")
		for _, c := range m.pkg.Conflicts {
			if conflictPkg := brew.GetPackage(c); conflictPkg != nil && conflictPkg.IsInstalled {
				b.WriteString(fmt.Sprintf("  %s %s\n", installedStyle.Render("✓"), c))
			} else {
				b.WriteString(fmt.Sprintf("  %s %s\n", uninstalledStyle.Render("✗"), c))
			}
		}
	}

	if len(m.pkg.Dependencies) > 0 {
		b.WriteString("\nDependencies:\n")
		for _, dep := range m.pkg.Dependencies {
			if depPkg := brew.GetPackage(dep); depPkg != nil && depPkg.IsInstalled {
				b.WriteString(fmt.Sprintf("  %s %s\n", installedStyle.Render("✓"), dep))
			} else {
				b.WriteString(fmt.Sprintf("  %s %s\n", uninstalledStyle.Render("✗"), dep))
				// For uninstall dependencies, show all recursive uninstalled dependencies
				recursiveMissingDeps := util.SortAndUniq(brew.GetRecursiveMissingDeps(dep))
				for _, d := range recursiveMissingDeps {
					if p := brew.GetPackage(d); !p.IsInstalled {
						b.WriteString(fmt.Sprintf("    %s %s\n", uninstalledStyle.Render("✗"), d))
					}
				}
			}
		}
	}

	if len(m.pkg.BuildDependencies) > 0 {
		b.WriteString("\nBuild Dependencies:\n")
		for _, dep := range m.pkg.BuildDependencies {
			if depPkg := brew.GetPackage(dep); depPkg != nil && depPkg.IsInstalled {
				b.WriteString(fmt.Sprintf("  %s %s\n", installedStyle.Render("✓"), dep))
			} else {
				b.WriteString(fmt.Sprintf("  %s %s\n", uninstalledStyle.Render("✗"), dep))
			}
		}
	}

	if len(m.pkg.Dependents) > 0 {
		b.WriteString("\nRequired By:\n")
		for _, dep := range m.pkg.Dependents {
			if depPkg := brew.GetPackage(dep); depPkg != nil && depPkg.IsInstalled {
				b.WriteString(fmt.Sprintf("  %s %s\n", installedStyle.Render("✓"), dep))
			} else {
				b.WriteString(fmt.Sprintf("  %s %s\n", uninstalledStyle.Render("✗"), dep))
			}
		}
	}

	m.vp.SetContent(lipgloss.NewStyle().Width(m.vp.Width).Render(b.String()))
	m.vp.GotoTop()
}
