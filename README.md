`taproom` is featured as "Tool of The Week" (July 15, 2025) on [Terminal Trove](https://terminaltrove.com/taproom/), the $HOME of all things in the terminal.

<a href="https://terminaltrove.com/taproom">
    <img src="https://cdn.terminaltrove.com/media/badges/tool_of_the_week/svg/terminal_trove_tool_of_the_week_gold_transparent.svg" alt="Terminal Trove Tool of The Week" width="640" />
</a>

# taproom

`taproom` is a cozy terminal user interface (TUI) for Homebrew. It provides a fast and fluid way to explore formulae and casks directly in your terminal.

![](https://raw.github.com/hzqtc/taproom/master/screenshot.png)

## ⭐ Highlights

`taproom` does a few things faster and easier than the `brew` cli:

- Search: searching in `taproom` gives results with all the details in real time; while using `brew` you need to run `brew search` then
  `brew info` on each package.
- Dependencies: `brew info` shows direct dependencies; while `taproom` also shows all recusive dependencies that are not installed - this is helpful
  to understand what exactly would be installed as dependencies
  - `taproom` also has clear indication for packages installed as dependencies vs installed explicitly
- Dependents: `brew uses --eval-all` shows dependents that require the target package but it is a pretty slow command; `taproom` have this information
  available without additional data loading
- Sorting: `taproom` supports sorting by popularity (90d installs) and size (disk spaced used)
- Navigation: 'h' opens an app's home page and 'b' opens the brew formula page

## ✨ Features

- **Table View:** Overview of all avaiable formulae and casks in Homebrew.
- **Detailed View:** Get more info on any package, including its description, version, homepage, license, dependencies, and 90-day install count.
  - Also shows dependencies recursively (only when the dependencies is not installed)
  - Also shows dependents (which other packages depends on this one)
- **Search:** Quickly find packages by name or description.
  - Default: match keyword in either name or description
  - Prefix `n:`: match the keyword only in name
  - Prefix `d:`: match the keyword only in description
- **Filtering:** View all packages, or filter by:
  - Formulae only
  - Casks only
  - Installed packages
  - Outdated packages
  - Packages you installed explicitly (not as dependencies)
- **Flexible Sorting:** Sort packages alphabetically by name or by 90-day popularity.
- **Status Indicators:** See at a glance which packages are installed, outdated, or pinned.
- **Execute brew commands:** upgrade, install, uninstall, pin or unpin packages directly in the TUI.

## 🚀 Getting Started

### Dependencies

- du (MacOS builtin command)
- [Homebrew](https://brew.sh/)
- A terminal emulator with a [nerd font](https://www.nerdfonts.com/)

### Install from pre-built binary

`gromgit@` (Thank you!) maintains a [formula](https://github.com/gromgit/homebrew-brewtils/blob/main/Formula/taproom.rb):

```
brew install gromgit/brewtils/taproom
```

Or use [`eget`](https://github.com/zyedidia/eget):

```
eget hzqtc/taproom
```

### Build from source

To build from source, follow these simple steps:

#### Build dependencies

- [Go](https://go.dev/doc/install)

#### Build

1.  Navigate to the project directory.
2.  Build the binary:
    ```sh
    make
    ```
3.  Run the application:
    ```sh
    ./taproom
    ```

## 🪄 Customization

The app's behavior can be further customized with command line flags:

- `--invalidate-cahce` or `-i` in short: immediately invalidate cache and re-download data from brew.sh
- `--hide-columns`: hide and skip loading data for specified columns
  - This can be helpful to further simplify the UI
  - While all data loading is done in parallel, some may be slower than others. This flag can be used to skip loading certain data to speed up app load
- `--load-timer` or `-t` in short: show a timer in the loading screen
- `--hide-help`: hide the help text on the bottom of the app
- `--sort-column` or `-s` in short: specify the column to sort by (this can still be changed in app with `s` and `S` keys)
- `--filters` or `-f` in short: specify initial filters (can still be changed later in the app)

Run `taproom -h` to learn more about the command line flags.

## 🛠️ Built With

- [Go](https://go.dev/)
- [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- [Bubbles](https://github.com/charmbracelet/bubbles)
- [Lip Gloss](https://github.com/charmbracelet/lipgloss)

## 🔀 Alternatives

`taproom` is inspired by [`boldbrew`](https://github.com/Valkyrie00/bold-brew) but it's not a clone.

- `taproom` has a different UI style
- `taproom` supports casks
- `taproom` supports sorting by different columns
- `taproom` shows missing dependencies recursively (i.e. dependencies of dependencies)
- `taproom` shows reverse dependencies (i.e. used by) that replaces an very slow `brew uses <formula> --eval-all` command
- `taproom` shows the size of installed packages

## ✨ Star History

[![Star History Chart](https://api.star-history.com/svg?repos=hzqtc/taproom&type=Date)](https://www.star-history.com/#hzqtc/taproom&Date)
