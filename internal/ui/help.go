package ui

import (
	"fmt"
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

	fmt.Fprintf(&b, "General   : ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("q"))
	fmt.Fprintf(&b, ": quit ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("R"))
	fmt.Fprintf(&b, ": refresh ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("tab"))
	fmt.Fprintf(&b, ": switch focus ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("/"))
	fmt.Fprintf(&b, ": search ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("esc"))
	fmt.Fprintf(&b, ": clear search ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("enter"))
	fmt.Fprintf(&b, ": exit search ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("s")+"/"+keyStyle.Render("S"))
	fmt.Fprintf(&b, ": sorting")
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, "Navigation: ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("j")+"/"+keyStyle.Render("↓"))
	fmt.Fprintf(&b, ": cursor down ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("k")+"/"+keyStyle.Render("↑"))
	fmt.Fprintf(&b, ": cursor up ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("PageUp"))
	fmt.Fprintf(&b, ": prev page ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("PageDown"))
	fmt.Fprintf(&b, ": next page ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("g"))
	fmt.Fprintf(&b, ": go to top ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("G"))
	fmt.Fprintf(&b, ": go to bottom")
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, "Filter    : ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("a"))
	fmt.Fprintf(&b, ": all (no filter) ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("f"))
	fmt.Fprintf(&b, ": formulae ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("c"))
	fmt.Fprintf(&b, ": casks ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("i"))
	fmt.Fprintf(&b, ": installed ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("o"))
	fmt.Fprintf(&b, ": outdated ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("e"))
	fmt.Fprintf(&b, ": explicitly installed ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("v"))
	fmt.Fprintf(&b, ": active")
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, "Commands  : ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("h"))
	fmt.Fprintf(&b, ": home page ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("b"))
	fmt.Fprintf(&b, ": brew.sh ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("r"))
	fmt.Fprintf(&b, ": release page ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("U"))
	fmt.Fprintf(&b, ": upgrade all ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("u"))
	fmt.Fprintf(&b, ": upgrade ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("t"))
	fmt.Fprintf(&b, ": install ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("x"))
	fmt.Fprintf(&b, ": uninstall ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("p"))
	fmt.Fprintf(&b, ": pin ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("P"))
	fmt.Fprintf(&b, ": unpin ")
	fmt.Fprintf(&b, "%s", keyStyle.Render("L"))
	fmt.Fprintf(&b, ": cleanup")

	return helpStyle.Render(b.String())
}
