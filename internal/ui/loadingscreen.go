package ui

import (
	"fmt"
	"strings"
	"taproom/internal/loading"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/pflag"
)

var (
	flagShowLoadTimer = pflag.BoolP("load-timer", "t", false, "Show a timer in the loading screen")
)

// Generated with 'figlet -f epic taproom'
const logo = `
_________ _______  _______  _______  _______  _______  _______
\__   __/(  ___  )(  ____ )(  ____ )(  ___  )(  ___  )(       )
   ) (   | (   ) || (    )|| (    )|| (   ) || (   ) || () () |
   | |   | (___) || (____)|| (____)|| |   | || |   | || || || |
   | |   |  ___  ||  _____)|     __)| |   | || |   | || |(_)| |
   | |   | (   ) || (      | (\ (   | |   | || |   | || |   | |
   | |   | )   ( || )      | ) \ \__| (___) || (___) || )   ( |
   )_(   |/     \||/       |/   \__/(_______)(_______)|/     \|
`

var (
	logoStyle = lipgloss.NewStyle().
			Foreground(highlightColor).
			Bold(true)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(highlightColor)
)

type LoadingScreenModel struct {
	progress  *loading.LoadingProgress
	isLoading bool
	errorMsg  string
	spinner   spinner.Model
	stopwatch stopwatch.Model
}

func NewLoadingScreenModel() LoadingScreenModel {
	s := spinner.New()
	s.Spinner = spinner.Dot

	var sw stopwatch.Model
	if *flagShowLoadTimer {
		sw = stopwatch.NewWithInterval(time.Millisecond)
	}

	return LoadingScreenModel{
		progress:  loading.NewLoadingProgress(),
		isLoading: true,
		spinner:   s,
		stopwatch: sw,
	}
}

func (m *LoadingScreenModel) Progress() *loading.LoadingProgress {
	return m.progress
}

func (m *LoadingScreenModel) StartLoading() tea.Cmd {
	m.isLoading = true
	m.errorMsg = ""
	cmds := []tea.Cmd{m.spinner.Tick}
	if *flagShowLoadTimer {
		cmds = append(cmds, m.stopwatch.Start())
	}
	return tea.Batch(cmds...)
}

func (m *LoadingScreenModel) StopLoading() tea.Cmd {
	var cmds []tea.Cmd
	m.isLoading = false
	m.progress.Reset()
	if *flagShowLoadTimer {
		cmds = append(cmds, m.stopwatch.Stop(), m.stopwatch.Reset())
	}
	return tea.Batch(cmds...)
}

func (m *LoadingScreenModel) SetError(err string) tea.Cmd {
	m.errorMsg = err
	return m.StopLoading()
}

func (m LoadingScreenModel) Update(msg tea.Msg) (LoadingScreenModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.isLoading {
			m.spinner, cmd = m.spinner.Update(msg)
		}
	case stopwatch.TickMsg:
		if m.isLoading {
			m.stopwatch, cmd = m.stopwatch.Update(msg)
		}
	case stopwatch.StartStopMsg, stopwatch.ResetMsg:
		m.stopwatch, cmd = m.stopwatch.Update(msg)
	}
	return m, cmd
}

func (m LoadingScreenModel) View() string {
	if m.errorMsg != "" {
		return fmt.Sprintf("An error occurred: %s\nPress 'q' to quit.", m.errorMsg)
	}

	if m.isLoading {
		var b strings.Builder
		m.spinner.Style = spinnerStyle
		b.WriteString(
			fmt.Sprintf(
				"%s\n%s\n\n%s Loading...",
				logoStyle.Render(logo),
				m.progress.String(logoStyle.Render("Done")),
				m.spinner.View(),
			),
		)
		if *flagShowLoadTimer {
			b.WriteString(m.stopwatch.View())
		}
		return b.String()
	}

	return ""
}
