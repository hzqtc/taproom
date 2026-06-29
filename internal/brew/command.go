package brew

import (
	"os/exec"
	"taproom/internal/data"

	tea "github.com/charmbracelet/bubbletea"
)

// --- Command Execution Messages ---

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

// runBrew runs a brew command by handing the terminal over to the child process via
// tea.ExecProcess. This is what makes interactive prompts work — most importantly sudo
// password entry for casks: Bubble Tea drops the alt-screen and restores the terminal to
// cooked mode, brew runs attached to the real terminal so it can prompt, and the TUI redraws
// once the command finishes.
//
// When pause is true, brew is wrapped in a small shell script that waits for Enter after the
// command exits so the user can read brew's output (caveats or errors) before the TUI
// repaints. "$@" passes args as a proper argv vector (no quoting/injection concerns) and
// `exit $status` preserves brew's real exit code for CommandFinishMsg.
func runBrew(command BrewCommand, pkgs []*data.Package, pause bool, args ...string) tea.Cmd {
	var cmd *exec.Cmd
	if pause {
		const script = `brew "$@"; status=$?; printf '\n[taproom] Press enter to return...'; read -r _; exit $status`
		// "sh" becomes $0; the package args follow as $1, $2, … consumed by "$@".
		shArgs := append([]string{"-c", script, "sh"}, args...)
		cmd = exec.Command("sh", shArgs...)
	} else {
		cmd = exec.Command("brew", args...)
	}
	// Stdin/Stdout/Stderr are left nil so ExecProcess connects them to the terminal.
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return CommandFinishMsg{Err: err, Command: command, Pkgs: pkgs}
	})
}

func UpgradeAllPackages(pkgs []*data.Package) tea.Cmd {
	return runBrew(BrewCommandUpgradeAll, pkgs, true, "upgrade")
}

func UpgradePackage(pkg *data.Package) tea.Cmd {
	args := []string{"upgrade"}
	if pkg.IsCask {
		args = append(args, "--cask")
	}
	args = append(args, pkg.Name)
	return runBrew(BrewCommandUpgrade, []*data.Package{pkg}, true, args...)
}

func InstallPackage(pkg *data.Package) tea.Cmd {
	args := []string{"install"}
	if pkg.IsCask {
		args = append(args, "--cask")
	}
	args = append(args, pkg.Name)
	return runBrew(BrewCommandInstall, []*data.Package{pkg}, true, args...)
}

func UninstallPackage(pkg *data.Package) tea.Cmd {
	args := []string{"uninstall"}
	if pkg.IsCask {
		args = append(args, "--cask")
	}
	args = append(args, pkg.Name)
	return runBrew(BrewCommandUninstall, []*data.Package{pkg}, true, args...)
}

func PinPackage(pkg *data.Package) tea.Cmd {
	return runBrew(BrewCommandPin, []*data.Package{pkg}, false, "pin", pkg.Name)
}

func UnpinPackage(pkg *data.Package) tea.Cmd {
	return runBrew(BrewCommandUnpin, []*data.Package{pkg}, false, "unpin", pkg.Name)
}

func Cleanup() tea.Cmd {
	return runBrew(BrewCommandCleanup, nil, true, "cleanup", "--prune=all")
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
