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
	"github.com/charmbracelet/x/ansi"
)

type DetailsPanelModel struct {
	pkg     *data.Package
	content string
	vp      viewport.Model
}

var (
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
	disabledSymbol            = "󰜺"
	deprecatedSymbol          = "󰀦"
	uninstalledSymbol         = "󰅖"
	installedSymbol           = "󰄬"
	explicitlyInstalledSymbol = "󰄭"
	outdatedSymbol            = "󰓦"
	pinnedSymbol              = "󰐃"
)

const (
	disabledSymbolAscii            = "DIS"
	deprecatedSymbolAscii          = "OLD"
	uninstalledSymbolAscii         = "---"
	installedSymbolAscii           = "DEP"
	explicitlyInstalledSymbolAscii = "INS"
	outdatedSymbolAscii            = "OUT"
	pinnedSymbolAscii              = "PIN"
)

func NewDetailsPanelModel() DetailsPanelModel {
	return DetailsPanelModel{}
}

func (m *DetailsPanelModel) SetDimension(width, height int) {
	m.vp.Width = width
	m.vp.Height = height
	m.vp.SetContent(lipgloss.NewStyle().Width(width).Render(m.content))
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

func formatStatus(pkg *data.Package) string {
	if data.NoNerdFont() {
		return pkg.Status()
	}
	return fmt.Sprintf("%s %s", formatStatusSymbol(pkg), pkg.Status())
}

func formatStatusSymbol(pkg *data.Package) string {
	if data.NoNerdFont() {
		if pkg.IsDisabled {
			return deprecatedStyle.Render(disabledSymbolAscii)
		} else if pkg.IsDeprecated {
			return deprecatedStyle.Render(deprecatedSymbolAscii)
		} else if pkg.IsPinned {
			return pinnedStyle.Render(pinnedSymbolAscii)
		} else if pkg.IsOutdated {
			return outdatedStyle.Render(outdatedSymbolAscii)
		} else if pkg.IsInstalled {
			if pkg.InstalledAsDependency {
				return installedStyle.Render(installedSymbolAscii)
			}
			return installedStyle.Render(explicitlyInstalledSymbolAscii)
		}
		return uninstalledStyle.Render(uninstalledSymbolAscii)
	}
	if pkg.IsDisabled {
		return deprecatedStyle.Render(disabledSymbol)
	} else if pkg.IsDeprecated {
		return deprecatedStyle.Render(deprecatedSymbol)
	} else if pkg.IsPinned {
		return pinnedStyle.Render(pinnedSymbol)
	} else if pkg.IsOutdated {
		return outdatedStyle.Render(outdatedSymbol)
	} else if pkg.IsInstalled {
		if pkg.InstalledAsDependency {
			return installedStyle.Render(installedSymbol)
		} else {
			return installedStyle.Render(explicitlyInstalledSymbol)
		}
	} else {
		return uninstalledStyle.Render(uninstalledSymbol)
	}
}

// Use OSC8 to wrap a string in a hyperlink. The id lets terminals underline the
// whole link on hover even when it wraps across multiple lines.
func hyperLink(url, text string) string {
	return ansi.SetHyperlink(url, "id=link") + text + ansi.ResetHyperlink()
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
	b.WriteString(fmt.Sprintf("Homepage: %s\n", hyperLink(m.pkg.Homepage, m.pkg.Homepage)))
	b.WriteString(fmt.Sprintf("License: %s\n", m.pkg.License))
	b.WriteString(fmt.Sprintf("Installs (90d): %d\n", m.pkg.Installs90d))

	b.WriteString(fmt.Sprintf("\nStatus: %s\n", formatStatus(m.pkg)))
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
			if p := brew.GetPackage(c); p != nil {
				b.WriteString(fmt.Sprintf("  %s %s\n", formatStatusSymbol(p), c))
			}
		}
	}

	if len(m.pkg.Dependencies) > 0 {
		b.WriteString("\nDependencies:\n")
		for _, dep := range m.pkg.Dependencies {
			depPkg := brew.GetPackage(dep)
			if depPkg == nil {
				continue
			}
			b.WriteString(fmt.Sprintf("  %s %s\n", formatStatusSymbol(depPkg), dep))
			if !depPkg.IsInstalled {
				// For uninstalled dependencies, show all recursive uninstalled dependencies
				recursiveDeps := util.SortAndUniq(brew.GetRecursiveMissingDeps(dep))
				for _, d := range recursiveDeps {
					if p := brew.GetPackage(d); p != nil && !p.IsInstalled {
						b.WriteString(fmt.Sprintf("    %s %s\n", formatStatusSymbol(p), d))
					}
				}
			}
		}
	}

	if len(m.pkg.BuildDependencies) > 0 {
		b.WriteString("\nBuild dependencies:\n")
		for _, dep := range m.pkg.BuildDependencies {
			if p := brew.GetPackage(dep); p != nil {
				b.WriteString(fmt.Sprintf("  %s %s\n", formatStatusSymbol(p), dep))
			}
		}
	}

	if len(m.pkg.Dependents) > 0 {
		b.WriteString("\nRequired By:\n")
		for _, dep := range m.pkg.Dependents {
			depPkg := brew.GetPackage(dep)
			if depPkg == nil {
				continue
			}
			b.WriteString(fmt.Sprintf("  %s %s\n", formatStatusSymbol(depPkg), dep))
			if depPkg.IsInstalled {
				// For installed dependents, show all recursive explicitly installed dependents
				recursiveDependents := util.SortAndUniq(brew.GetRecursiveInstalledDependents(dep))
				for _, d := range recursiveDependents {
					if p := brew.GetPackage(d); p != nil && p.IsInstalled && !p.InstalledAsDependency {
						b.WriteString(fmt.Sprintf("    %s %s\n", formatStatusSymbol(p), d))
					}
				}
			}
		}
	}

	m.content = b.String()
	m.vp.SetContent(lipgloss.NewStyle().Width(m.vp.Width).Render(m.content))
	m.vp.GotoTop()
}
