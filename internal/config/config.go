// Package config loads and validates the post-install YAML config.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

type Config struct {
	// Helper is the package manager used by the generated `mgr` function.
	Helper string  `yaml:"helper"`
	Groups []Group `yaml:"groups"`
	// After is a shell snippet run after every selected item.
	After string `yaml:"after"`
}

type Group struct {
	Name   string  `yaml:"name"`
	Groups []Group `yaml:"groups"`
	Items  []Item  `yaml:"items"`
}

// Item is one selectable entry. A bare string in YAML is shorthand for
// an item that installs the package of the same name.
type Item struct {
	Name     string
	Enabled  bool
	Packages []string
	Run      string
}

var Helpers = []string{"yay", "paru", "pacman"}

var pkgName = regexp.MustCompile(`^[a-zA-Z0-9@._+-]+$`)

func (it *Item) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.ScalarNode:
		if n.Value == "" {
			return fmt.Errorf("line %d: empty item", n.Line)
		}
		*it = Item{Name: n.Value, Enabled: true, Packages: []string{n.Value}}
		return nil
	case yaml.MappingNode:
		if err := checkKeys(n, "name", "enabled", "packages", "run"); err != nil {
			return err
		}
		var raw struct {
			Name     string   `yaml:"name"`
			Enabled  *bool    `yaml:"enabled"`
			Packages []string `yaml:"packages"`
			Run      string   `yaml:"run"`
		}
		if err := n.Decode(&raw); err != nil {
			return err
		}
		if raw.Name == "" {
			return fmt.Errorf("line %d: item without name", n.Line)
		}
		*it = Item{Name: raw.Name, Enabled: true, Packages: raw.Packages, Run: raw.Run}
		if raw.Enabled != nil {
			it.Enabled = *raw.Enabled
		}
		// Without an explicit script or package list the name is the package.
		if it.Packages == nil && it.Run == "" {
			it.Packages = []string{it.Name}
		}
		return nil
	default:
		return fmt.Errorf("line %d: item must be a package name or a mapping", n.Line)
	}
}

// checkKeys rejects unknown keys. Decoder.KnownFields does not reach into
// custom unmarshalers, so items check their keys by hand.
func checkKeys(n *yaml.Node, allowed ...string) error {
	for i := 0; i < len(n.Content); i += 2 {
		k := n.Content[i]
		if !slices.Contains(allowed, k.Value) {
			return fmt.Errorf("line %d: unknown key %q (allowed: %s)", k.Line, k.Value, strings.Join(allowed, ", "))
		}
	}
	return nil
}

func Parse(data []byte) (*Config, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var c Config
	if err := dec.Decode(&c); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("config is empty")
		}
		return nil, err
	}
	if c.Helper == "" {
		c.Helper = "yay"
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) validate() error {
	if !slices.Contains(Helpers, c.Helper) {
		return fmt.Errorf("helper %q is not supported (use one of: %s)", c.Helper, strings.Join(Helpers, ", "))
	}
	return validateGroups(c.Groups, "")
}

func validateGroups(groups []Group, path string) error {
	for _, g := range groups {
		if g.Name == "" {
			return fmt.Errorf("%sgroup without name", path)
		}
		p := path + g.Name + ": "
		for _, it := range g.Items {
			for _, pkg := range it.Packages {
				if !pkgName.MatchString(pkg) {
					return fmt.Errorf("%s%s: invalid package name %q", p, it.Name, pkg)
				}
			}
		}
		if err := validateGroups(g.Groups, p); err != nil {
			return err
		}
	}
	return nil
}

// Load reads a config from a file path, an http(s):// or file:// URL, or "-" for stdin.
func Load(src string) (*Config, error) {
	data, err := read(src)
	if err != nil {
		return nil, err
	}
	c, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", src, err)
	}
	return c, nil
}

func read(src string) ([]byte, error) {
	switch {
	case src == "-":
		return io.ReadAll(os.Stdin)
	case strings.HasPrefix(src, "http://"), strings.HasPrefix(src, "https://"):
		client := http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(src)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s: %s", src, resp.Status)
		}
		return io.ReadAll(resp.Body)
	default:
		return os.ReadFile(strings.TrimPrefix(src, "file://"))
	}
}
