// Command arch_post_install picks post-install steps from a YAML config
// and turns them into a bash script.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/FelixDes/arch_post_install/internal/config"
	"github.com/FelixDes/arch_post_install/internal/script"
	"github.com/FelixDes/arch_post_install/internal/tui"
)

func main() {
	if err := run(); err != nil {
		if errors.Is(err, tui.ErrAborted) {
			os.Exit(130)
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	out := flag.String("o", "", "write the script to `file` instead of stdout")
	execute := flag.Bool("run", false, "run the script after confirmation")
	yes := flag.Bool("y", false, "with -run: do not ask for confirmation")
	defaults := flag.Bool("defaults", false, "skip the picker and take items enabled in the config")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] CONFIG\n\nCONFIG is a file path, an http(s):// URL or - for stdin.\n\nFlags:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if *execute && os.Geteuid() == 0 {
		return errors.New("do not run as root: the script calls sudo itself and AUR helpers refuse root")
	}

	cfg, err := config.Load(flag.Arg(0))
	if err != nil {
		return err
	}

	var items []*config.Item
	if *defaults {
		items = enabledItems(cfg.Groups)
	} else if items, err = tui.Run(cfg); err != nil {
		return err
	}
	text := script.Render(cfg.Helper, items, cfg.After)

	if !*execute {
		if *out == "" {
			fmt.Print(text)
			return nil
		}
		if err := os.WriteFile(*out, []byte(text), 0o755); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "script written to", *out)
		return nil
	}

	path := *out
	if path == "" {
		f, err := os.CreateTemp("", "arch_post_install-*.sh")
		if err != nil {
			return err
		}
		path = f.Name()
		defer os.Remove(path)
		f.Close()
	}
	if err := os.WriteFile(path, []byte(text), 0o755); err != nil {
		return err
	}
	if !*yes {
		fmt.Fprint(os.Stderr, text)
		ok, err := confirm(fmt.Sprintf("\nRun this script (%d items)?", len(items)))
		if err != nil || !ok {
			return err
		}
	}
	cmd := exec.Command("bash", path)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func enabledItems(groups []config.Group) []*config.Item {
	var out []*config.Item
	for i := range groups {
		out = append(out, enabledItems(groups[i].Groups)...)
		for j := range groups[i].Items {
			if it := &groups[i].Items[j]; it.Enabled {
				out = append(out, it)
			}
		}
	}
	return out
}

// confirm asks on the terminal, since stdin may be the config.
func confirm(question string) (bool, error) {
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return false, err
	}
	defer tty.Close()
	fmt.Fprint(os.Stderr, question+" [y/N] ")
	answer, err := bufio.NewReader(tty).ReadString('\n')
	if err != nil {
		return false, err
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}
