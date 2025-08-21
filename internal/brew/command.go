package brew

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"taproom/internal/data"
	"taproom/internal/util"

	tea "github.com/charmbracelet/bubbletea"
)

// --- Command Execution Messages ---

type CommandStartMsg struct{}
type CommandOutputMsg struct {
	Ch   chan tea.Msg
	Line string
}
type CommandFinishMsg struct {
	Err     error
	Command BrewCommand
	Pkgs    []*data.Package
}

type BrewCommand string

const (
	BrewCommandUpgradeAll BrewCommand = "upgradeAll"
	BrewCommandUpgrade    BrewCommand = "upgrade"
	BrewCommandInstall    BrewCommand = "install"
	BrewCommandUninstall  BrewCommand = "uninstall"
	BrewCommandPin        BrewCommand = "pin"
	BrewCommandUnpin      BrewCommand = "unpin"
	BrewCommandCleanup    BrewCommand = "cleanup"
)

// --- Command Functions ---

func startCommand() tea.Cmd {
	return func() tea.Msg {
		return CommandStartMsg{}
	}
}

func StreamOutput(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func feedOutput(ch chan tea.Msg, pipe io.ReadCloser) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		ch <- CommandOutputMsg{Ch: ch, Line: scanner.Text()}
	}
}

func execute(BrewCommand BrewCommand, pkgs []*data.Package, args ...string) tea.Cmd {
	return func() tea.Msg {
		ch := make(chan tea.Msg)

		go func() {
			defer close(ch)

			cmdLine := fmt.Sprintf("brew %s", strings.Join(args, " "))

			if BrewCommand == BrewCommandInstall || BrewCommand == BrewCommandUninstall {
				if pkg := pkgs[0]; !pkg.InstallSupported {
					ch <- CommandOutputMsg{Ch: ch, Line: fmt.Sprintf("%s can’t be %sed because it’s a .pkg and may need sudo", pkg.Name, BrewCommand)}
					ch <- CommandOutputMsg{Ch: ch, Line: fmt.Sprintf("please run '%s' in command line", cmdLine)}
					ch <- CommandFinishMsg{Err: fmt.Errorf("install not supported")}
					return
				}
			}

			ch <- CommandOutputMsg{Ch: ch, Line: "> " + cmdLine}
			cmd := exec.Command("brew", args...)
			// Connect to stdout and stderr
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				ch <- CommandFinishMsg{Err: fmt.Errorf("failed to get stdout pipe: %w", err)}
				return
			}
			stderr, err := cmd.StderrPipe()
			if err != nil {
				ch <- CommandFinishMsg{Err: fmt.Errorf("failed to get stderr pipe: %w", err)}
				return
			}
			// Start command
			if err := cmd.Start(); err != nil {
				ch <- CommandFinishMsg{Err: fmt.Errorf("failed to start command: %w", err)}
				return
			}

			var wg sync.WaitGroup
			wg.Add(2)
			// Stream stdout and stderr
			go func() {
				defer wg.Done()
				feedOutput(ch, stdout)
			}()
			go func() {
				defer wg.Done()
				feedOutput(ch, stderr)
			}()

			cmdErr := cmd.Wait()
			wg.Wait()
			ch <- CommandFinishMsg{Err: cmdErr, Command: BrewCommand, Pkgs: pkgs}
		}()

		return CommandOutputMsg{Ch: ch}
	}
}

func UpgradeAllPackages(pkgs []*data.Package) tea.Cmd {
	return tea.Batch(startCommand(), execute(BrewCommandUpgradeAll, pkgs, "upgrade"))
}

func UpgradePackage(pkg *data.Package) tea.Cmd {
	args := []string{"upgrade"}
	if pkg.IsCask {
		args = append(args, "--cask")
	}
	args = append(args, pkg.Name)
	return tea.Batch(startCommand(), execute(BrewCommandUpgrade, []*data.Package{pkg}, args...))
}

func InstallPackage(pkg *data.Package) tea.Cmd {
	args := []string{"install"}
	if pkg.IsCask {
		args = append(args, "--cask")
	}
	args = append(args, pkg.Name)
	return tea.Batch(startCommand(), execute(BrewCommandInstall, []*data.Package{pkg}, args...))
}

func UninstallPackage(pkg *data.Package) tea.Cmd {
	args := []string{"uninstall"}
	if pkg.IsCask {
		args = append(args, "--cask")
	}
	args = append(args, pkg.Name)
	return tea.Batch(startCommand(), execute(BrewCommandUninstall, []*data.Package{pkg}, args...))
}

func PinPackage(pkg *data.Package) tea.Cmd {
	return tea.Batch(startCommand(), execute(BrewCommandPin, []*data.Package{pkg}, "pin", pkg.Name))
}

func UnpinPackage(pkg *data.Package) tea.Cmd {
	return tea.Batch(startCommand(), execute(BrewCommandUnpin, []*data.Package{pkg}, "unpin", pkg.Name))
}

func Cleanup() tea.Cmd {
	return tea.Batch(startCommand(), execute(BrewCommandCleanup, []*data.Package{}, "cleanup", "--prune=all"))
}

func UpdatePackageForAction(command BrewCommand, pkgs []*data.Package) {
	switch command {
	case BrewCommandUpgradeAll, BrewCommandUpgrade:
		for _, pkg := range pkgs {
			pkg.MarkInstalled()
		}
	case BrewCommandInstall:
		for _, pkg := range pkgs {
			pkg.MarkInstalled()
			// Also mark uninstalled dependencies as installed
			for _, depName := range GetRecursiveMissingDeps(pkg.Name) {
				GetPackage(depName).MarkInstalled()
			}

			pkg.Size = fetchPackageSize(pkg)
			pkg.FormattedSize = util.FormatSize(pkg.Size)
		}
	case BrewCommandUninstall:
		for _, pkg := range pkgs {
			pkg.MarkUninstalled()
		}
	case BrewCommandPin:
		for _, pkg := range pkgs {
			pkg.MarkPinned()
		}
	case BrewCommandUnpin:
		for _, pkg := range pkgs {
			pkg.MarkUnpinned()
		}
	}
}
