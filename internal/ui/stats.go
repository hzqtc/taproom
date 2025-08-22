package ui

import (
	"fmt"
	"taproom/internal/data"
	"taproom/internal/util"

	"github.com/charmbracelet/lipgloss"
)

type StatsModel struct {
	pkgs []*data.Package
}

var statsStyle = lipgloss.NewStyle().
	Padding(1 /* top */, 2 /* horizontal */, 0 /* bottom */)

func NewStatsModel() StatsModel {
	return StatsModel{}
}

func (m *StatsModel) SetPackages(pkgs []*data.Package) {
	m.pkgs = pkgs
}

func (m *StatsModel) SetWidth(w int) {
	statsStyle = statsStyle.Width(w)
}

func (m StatsModel) View() string {
	var formulaeNum, casksNum int
	var installedFormulaeNum, installedFormulaeDepNum, installedCasksNum int
	var formulaeSize, casksSize int64
	for _, pkg := range m.pkgs {
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
	return statsStyle.Render(
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
