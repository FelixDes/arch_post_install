# Arch Post-Install Tool

A command-line utility for Arch-based Linux distributions to automate post-installation tasks. It provides an interactive ncurses-based menu to select and execute installation commands for packages using `yay` or custom shell scripts, configured via a YAML file.

## Features
- **Interactive Menu**: Navigate and select installation tasks using a terminal-based interface.
- **YAML Configuration**: Define packages and custom shell commands in a structured YAML file, supporting local files or remote URLs.
- **Flexible Actions**: Install AUR packages with `yay` or run custom shell commands.
- **Script Generation**: Generate and optionally execute shell scripts from selected actions.
- **Command-Line Options**: Supports options for specifying YAML input, executing scripts, and saving output to files.

## Requirements
- C++26 compiler
- CMake 3.31+
- Libraries: `libcurl`, `ncurses`, `yaml-cpp`, `cxxopts`
- AUR helper: `yay`

## Building
```bash
mkdir build
cd build
cmake ..
cmake --build .
```

Binaries are placed in `build/bin`.

## Usage
```bash
./build/bin/arch_post_install -f <yaml_file_or_url> [-e] [-w [output_file]]
```

- `-f, --file <file|url>`: Path or URL to the YAML configuration file.
- `-e, --exec`: Execute the generated script.
- `-w, --write [filename]`: Save the script to a file (default: auto-generated name).
- `-h, --help`: Display help message.

## Example YAML
```yaml
sections:
  core:
    items:
      - vim
      - name: custom_setup
        enabled: true
        commands:
          - __MGR__ -S neovim # alias for `yay --noconfirm --answerdiff=None --answeredit=None`
          - sudo systemctl enable sshd
  dev:
    items:
      - git
```

## Installation
Binaries are available in the [GitHub Packages registry](https://github.com/FelixDes/arch_post_install/packages).

## License
MIT License