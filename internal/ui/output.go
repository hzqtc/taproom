package ui

import "strings"

type OutputModel struct {
	lines    []string
	hasError bool
}

var outputStyle = baseStyle.
	Margin(1 /* top */, 0 /* horizontal */, 0 /* bottom */).
	Padding(0, 1)

const outputMaxLines = 10

func NewOutputModel() OutputModel {
	return OutputModel{}
}

func (m *OutputModel) Clear() {
	m.lines = []string{}
	m.hasError = false
}

func (m *OutputModel) Append(l string) {
	m.lines = append(m.lines, l)
}

func (m *OutputModel) SetError() {
	m.hasError = true
}

func (m *OutputModel) SetWidth(w int) {
	outputStyle = outputStyle.Width(w)
}

func (m OutputModel) View() string {
	if len(m.lines) == 0 {
		return ""
	}

	var output string
	if len(m.lines) > outputMaxLines {
		output = strings.Join(m.lines[len(m.lines)-outputMaxLines:], "\n")
	} else {
		output = strings.Join(m.lines, "\n")
	}

	if m.hasError {
		return outputStyle.BorderForeground(errBorderColor).Render(output)
	} else {
		return outputStyle.Render(output)
	}
}
