package script

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/FelixDes/arch_post_install/internal/config"
)

func TestRender(t *testing.T) {
	items := []*config.Item{
		{Name: "git", Packages: []string{"git"}},
		{Name: "docker", Packages: []string{"docker", "git"}, Run: "sudo systemctl enable docker\n"},
		{Name: "it's", Run: "cat <<'EOF'\n  indented\nEOF"},
	}
	got := Render("yay", items, "echo done")

	for _, want := range []string{
		"set -euo pipefail",
		"mgr() { yay -S --needed",
		"if ! command -v yay",
		"mgr \\\n  git \\\n  docker\n",
		"step 'docker'\n(\nsudo systemctl enable docker\n)\n",
		"step 'it'\\''s'\n(\ncat <<'EOF'\n  indented\nEOF\n)\n",
		"step 'After'\n(\necho done\n)\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Count(got, "\n  git \\\n") != 1 {
		t.Errorf("package git not deduplicated:\n%s", got)
	}

	if bash, err := exec.LookPath("bash"); err == nil {
		cmd := exec.Command(bash, "-n")
		cmd.Stdin = strings.NewReader(got)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("bash -n: %v\n%s", err, out)
		}
	}
}

func TestRenderPacmanNoBootstrap(t *testing.T) {
	got := Render("pacman", nil, "")
	if strings.Contains(got, "command -v") {
		t.Errorf("pacman needs no bootstrap:\n%s", got)
	}
	if strings.Contains(got, "Install packages") {
		t.Errorf("no packages selected, no install step expected:\n%s", got)
	}
}
