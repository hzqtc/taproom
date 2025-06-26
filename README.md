# taproom

`taproom` is a cozy terminal user interface (TUI) for Homebrew. It provides a fast and fluid way to explore formulae and casks directly in your
terminal. `taproom` is inspired by [`boldbrew`](https://github.com/Valkyrie00/bold-brew).

## ✨ Features

*   **Table View:** All or selected formula or cask.
*   **Search:** Quickly find packages by name or description.
*   **Detailed View:** Get more info on any package, including its description, version, homepage, license, dependencies, and 90-day install count.
*   **Smart Filtering:** View all packages, or filter by:
    *   Formulae only
    *   Casks only
    *   Installed packages
    *   Outdated packages
    *   Packages you installed explicitly (not as dependencies)
*   **Flexible Sorting:** Sort packages alphabetically by name or by 90-day popularity.
*   **Status Indicators:** See at a glance which packages are installed, outdated, or pinned.
*   **Responsive Layout:** The UI adapts to your terminal window size.

## ⌨️ Keybindings

| Key(s)      | Action                         |
| :---------- | :----------------------------- |
| `↑`/`k`       | Move Up                        |
| `↓`/`j`       | Move Down                      |
| `pgup`      | Page Up                        |
| `pgdn`      | Page Down                      |
| `g`/`home`    | Go to Top                      |
| `G`/`end`     | Go to Bottom                   |
| `/`         | Focus Search Bar               |
| `enter`     | Exit Search                    |
| `esc`       | Clear/Exit Search              |
| `s`         | Toggle Sort (Name/Popularity)  |
| `a`         | Show All Packages              |
| `f`         | Filter: Formulae Only          |
| `c`         | Filter: Casks Only             |
| `i`         | Filter: Installed              |
| `o`         | Filter: Outdated               |
| `e`         | Filter: Explicitly Installed   |
| `?`         | Toggle Help View               |
| `q`/`ctrl+c`  | Quit                           |

## 🚀 Getting Started

To get a local copy up and running, follow these simple steps.

### Prerequisites

*   [Go](https://go.dev/doc/install)
*   [Homebrew](https://brew.sh/) (for fetching package information)

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
