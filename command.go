package main

import (
	"bufio"
	"fmt"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

// --- Command Execution Messages ---

type commandStartMsg struct{}
type commandOutputMsg struct {
	ch   chan tea.Msg
	line string
}
type commandFinishMsg struct {
	err    error
	action commandAction
	pkgs   []*Package
}

type commandAction string

const (
	actionUpgradeAll commandAction = "upgradeAll"
	actionUpgrade    commandAction = "upgrade"
	actionInstall    commandAction = "install"
	actionUninstall  commandAction = "uninstall"
	actionPin        commandAction = "pin"
	actionUnpin      commandAction = "unpin"
)

// --- Command Functions ---

func startCommand() tea.Cmd {
	return func() tea.Msg {
		return commandStartMsg{}
	}
}

func streamOutput(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func execute(action commandAction, pkgs []*Package, args ...string) tea.Cmd {
	return func() tea.Msg {
		ch := make(chan tea.Msg)

		go func() {
			defer close(ch)
			cmd := exec.Command("brew", args...)

			stdout, err := cmd.StdoutPipe()
			if err != nil {
				ch <- commandFinishMsg{err: fmt.Errorf("failed to get stdout pipe: %w", err), action: action}
				return
			}

			if err := cmd.Start(); err != nil {
				ch <- commandFinishMsg{err: fmt.Errorf("failed to start command: %w", err), action: action}
				return
			}

			output := make(chan struct{})
			go func() {
				scanner := bufio.NewScanner(stdout)
				for scanner.Scan() {
					ch <- commandOutputMsg{ch: ch, line: scanner.Text()}
				}
				close(output)
			}()
			<-output

			cmdErr := cmd.Wait()
			ch <- commandFinishMsg{err: cmdErr, action: action, pkgs: pkgs}
		}()

		return commandOutputMsg{ch: ch}
	}
}

func upgradeAllPackages(pkgs []*Package) tea.Cmd {
	return tea.Batch(startCommand(), execute(actionUpgradeAll, pkgs, "upgrade"))
}

func upgradePackage(pkg *Package) tea.Cmd {
	args := []string{"upgrade"}
	if pkg.IsCask {
		args = append(args, "--cask")
	}
	args = append(args, pkg.Name)
	return tea.Batch(startCommand(), execute(actionUpgrade, []*Package{pkg}, args...))
}

func installPackage(pkg *Package) tea.Cmd {
	args := []string{"install"}
	if pkg.IsCask {
		args = append(args, "--cask")
	}
	args = append(args, pkg.Name)
	return tea.Batch(startCommand(), execute(actionInstall, []*Package{pkg}, args...))
}

func uninstallPackage(pkg *Package) tea.Cmd {
	args := []string{"uninstall"}
	if pkg.IsCask {
		args = append(args, "--cask")
	}
	args = append(args, pkg.Name)
	return tea.Batch(startCommand(), execute(actionUninstall, []*Package{pkg}, args...))
}

func pinPackage(pkg *Package) tea.Cmd {
	return tea.Batch(startCommand(), execute(actionPin, []*Package{pkg}, "pin", pkg.Name))
}

func unpinPackage(pkg *Package) tea.Cmd {
	return tea.Batch(startCommand(), execute(actionUnpin, []*Package{pkg}, "unpin", pkg.Name))
}
