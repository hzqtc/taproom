# taproom

`taproom` is a cozy terminal user interface (TUI) for Homebrew. It provides a fast and fluid way to explore formulae and casks directly in your
terminal. `taproom` is inspired by [`boldbrew`](https://github.com/Valkyrie00/bold-brew).

## ✨ Features

*   **Table View:** All or selected formula or cask.
*   **Search:** Quickly find packages by name or description.
*   **Detailed View:** Get more info on any package, including its description, version, homepage, license, dependencies, and 90-day install count.
    * Also shows dependencies recursively (only when the dependencies is not installed)
    * Also shows dependents (which other packages depends on this one)
*   **Filtering:** View all packages, or filter by:
    *   Formulae only
    *   Casks only
    *   Installed packages
    *   Outdated packages
    *   Packages you installed explicitly (not as dependencies)
*   **Flexible Sorting:** Sort packages alphabetically by name or by 90-day popularity.
*   **Status Indicators:** See at a glance which packages are installed, outdated, or pinned.
*   **Execute brew commands:** upgrade, install, uninstall, pin or unpin packages directly in the TUI.

## 🚀 Getting Started

To get a local copy up and running, follow these simple steps.

### Prerequisites

*   [Go](https://go.dev/doc/install)
*   [Homebrew](https://brew.sh/) (for fetching package information)

### Run

```sh
go install github.com/hzqtc/taproom
```

### Build

1.  Navigate to the project directory.
2.  Build the binary:
    ```sh
    make
    ```
3.  Run the application:
    ```sh
    ./taproom
    ```

## 🛠️ Built With

*   [Go](https://go.dev/)
*   [Bubble Tea](https://github.com/charmbracelet/bubbletea)
*   [Bubbles](https://github.com/charmbracelet/bubbles)
*   [Lip Gloss](https://github.com/charmbracelet/lipgloss)
