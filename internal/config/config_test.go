package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	c, err := Parse([]byte(`
groups:
  - name: A
    groups:
      - name: B
        items: [vim]
    items:
      - git
      - name: docker
        enabled: false
        packages: [docker, docker-compose]
        run: sudo systemctl enable docker
      - name: setup
        run: echo hi
after: echo done
`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Helper != "yay" {
		t.Errorf("default helper = %q", c.Helper)
	}
	want := []Item{
		{Name: "git", Enabled: true, Packages: []string{"git"}},
		{Name: "docker", Enabled: false, Packages: []string{"docker", "docker-compose"}, Run: "sudo systemctl enable docker"},
		{Name: "setup", Enabled: true, Run: "echo hi"},
	}
	if got := c.Groups[0].Items; !reflect.DeepEqual(got, want) {
		t.Errorf("items:\n got %+v\nwant %+v", got, want)
	}
	if got := c.Groups[0].Groups[0].Items[0].Name; got != "vim" {
		t.Errorf("nested item = %q", got)
	}
	if c.After != "echo done" {
		t.Errorf("after = %q", c.After)
	}
}

func TestParseErrors(t *testing.T) {
	tests := map[string]struct{ yaml, want string }{
		"empty":             {``, "config is empty"},
		"unknown top key":   {"sections: {}", "field sections not found"},
		"unknown group key": {"groups: [{name: A, item: []}]", "field item not found"},
		"unknown item key":  {"groups: [{name: A, items: [{name: x, commands: y}]}]", `unknown key "commands"`},
		"empty item":        {"groups: [{name: A, items: [git, '']}]", "empty item"},
		"item no name":      {"groups: [{name: A, items: [{run: ls}]}]", "item without name"},
		"group no name":     {"groups: [{items: [git]}]", "group without name"},
		"bad package":       {"groups: [{name: A, items: ['git; rm -rf /']}]", "invalid package name"},
		"bad helper":        {"helper: apt", `helper "apt" is not supported`},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestLoadBundledConfigs(t *testing.T) {
	files, _ := filepath.Glob("../../configs/*.yaml")
	if len(files) == 0 {
		t.Fatal("no configs found")
	}
	for _, f := range files {
		if _, err := Load(f); err != nil {
			t.Error(err)
		}
	}
}

func TestLoadFileURL(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(p, []byte("groups: [{name: A, items: [git]}]"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("file://" + p); err != nil {
		t.Fatal(err)
	}
}
