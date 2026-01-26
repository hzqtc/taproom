package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type HelpModel struct {
}

var helpStyle = lipgloss.NewStyle().
	Padding(1 /* top */, 2 /* horizontal */, 0 /* bottom */)

func NewHelpModel() HelpModel {
	return HelpModel{}
}

func (m *HelpModel) SetWidth(w int) {
	headerStyle = headerStyle.Width(w)
}

func (m HelpModel) View() string {
	var b strings.Builder

	b.WriteString("General   : ")
	b.WriteString(keyStyle.Render("q"))
	b.WriteString(": quit ")
	b.WriteString(keyStyle.Render("R"))
	b.WriteString(": refresh ")
	b.WriteString(keyStyle.Render("tab"))
	b.WriteString(": switch focus ")
	b.WriteString(keyStyle.Render("/"))
	b.WriteString(": search ")
	b.WriteString(keyStyle.Render("esc"))
	b.WriteString(": clear search ")
	b.WriteString(keyStyle.Render("enter"))
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
	b.WriteString(": active ")
	b.WriteString(keyStyle.Render("m"))
	b.WriteString(": compatible")
	b.WriteString("\n")
	b.WriteString("Commands  : ")
	b.WriteString(keyStyle.Render("h"))
	b.WriteString(": home page ")
	b.WriteString(keyStyle.Render("b"))
	b.WriteString(": brew.sh ")
	b.WriteString(keyStyle.Render("r"))
	b.WriteString(": release page ")
	b.WriteString(keyStyle.Render("U"))
	b.WriteString(": upgrade all ")
	b.WriteString(keyStyle.Render("u"))
	b.WriteString(": upgrade ")
	b.WriteString(keyStyle.Render("t"))
	b.WriteString(": install ")
	b.WriteString(keyStyle.Render("x"))
	b.WriteString(": uninstall ")
	b.WriteString(keyStyle.Render("p"))
	b.WriteString(": pin ")
	b.WriteString(keyStyle.Render("P"))
	b.WriteString(": unpin ")
	b.WriteString(keyStyle.Render("L"))
	b.WriteString(": cleanup")

	return helpStyle.Render(b.String())
}
