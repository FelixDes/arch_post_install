# Arch Post-Install Tool

Pick what to install on a fresh Arch-based system from a YAML config, get a bash script.

- Tree picker in the terminal: toggle single items or whole groups.
- All packages go into one `yay -S --needed` call, then each item's snippet runs as its own step.
- The script stops on the first error (`set -euo pipefail`) and bootstraps `yay`/`paru` if it is missing.
- One static binary, no runtime dependencies.

## Quick start

On a fresh system, as a regular user:

```bash
curl -fsSL https://github.com/FelixDes/arch_post_install/releases/latest/download/arch_post_install -o /tmp/apo \
  && chmod +x /tmp/apo \
  && /tmp/apo -run https://raw.githubusercontent.com/FelixDes/arch_post_install/master/configs/felix.yaml
```

## Install

Download `arch_post_install` from [Releases](https://github.com/FelixDes/arch_post_install/releases), or build:

```bash
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' .
```

## Usage

```bash
arch_post_install [flags] CONFIG
```

`CONFIG` is a file path, an `http(s)://` / `file://` URL, or `-` for stdin.

| Flag | |
|---|---|
| *(none)* | print the script to stdout |
| `-o FILE` | write the script to `FILE` |
| `-run` | show the script, ask, then run it (with `-o`, the script is kept) |
| `-y` | with `-run`: don't ask |
| `-defaults` | skip the picker, take items as `enabled` in the config |

Picker keys: `↑↓`/`jk` move, `space` toggle item or group, `a` toggle all, `→←`/`lh` open/close group, `enter` done, `q`/`esc` cancel.

## Config

```yaml
helper: yay            # yay (default), paru or pacman

groups:
  - name: Shell
    items:
      - zsh            # install package "zsh"
      - name: oh-my-zsh
        packages: [git, curl]
        run: |         # runs after all packages are installed
          RUNZSH=no sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)"
    groups:            # groups nest
      - name: Extras
        items:
          - name: alacritty
            enabled: false   # unchecked by default
            run: |
              mgr alacritty
              notify "Post-install" "alacritty installed"

after: |               # runs last
  fastfetch
```

An item is either a package name or a mapping with `name` and optional `enabled` (default `true`), `packages` and `run`. A mapping with neither `packages` nor `run` installs the package called `name`.

Snippets run in a subshell each, so `cd` and variables don't leak between steps. They can call:

- `mgr <pkg>...` — install packages with the configured helper
- `notify <title> <text>` — desktop notification

Unknown keys are errors, so typos don't get silently ignored. See [`configs/felix.yaml`](configs/felix.yaml) for a full example.

## License

MIT
