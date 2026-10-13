// Package tui is the interactive tree for picking config items.
package tui

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/FelixDes/arch_post_install/internal/config"
)

var ErrAborted = errors.New("aborted")

type node struct {
	name     string
	item     *config.Item // nil for groups
	checked  bool
	expanded bool
	depth    int
	parent   *node
	children []*node
}

func (n *node) isGroup() bool { return n.item == nil }

// count returns how many items under n are checked, and how many there are.
func (n *node) count() (checked, total int) {
	if !n.isGroup() {
		if n.checked {
			return 1, 1
		}
		return 0, 1
	}
	for _, c := range n.children {
		ch, t := c.count()
		checked += ch
		total += t
	}
	return checked, total
}

func (n *node) setChecked(v bool) {
	n.checked = v
	for _, c := range n.children {
		c.setChecked(v)
	}
}

func buildGroups(groups []config.Group, parent *node, depth int) []*node {
	var nodes []*node
	for i := range groups {
		g := &groups[i]
		gn := &node{name: g.Name, depth: depth, parent: parent}
		gn.children = buildGroups(g.Groups, gn, depth+1)
		for j := range g.Items {
			it := &g.Items[j]
			gn.children = append(gn.children, &node{name: it.Name, item: it, checked: it.Enabled, depth: depth + 1, parent: gn})
		}
		nodes = append(nodes, gn)
	}
	return nodes
}

type Model struct {
	root    *node
	rows    []*node // visible nodes in display order
	cursor  int
	offset  int
	width   int
	height  int
	done    bool
	aborted bool
	styles  styles
}

type styles struct {
	title, cursor, group, dim, help lipgloss.Style
}

func New(cfg *config.Config) *Model {
	root := &node{expanded: true, depth: -1}
	root.children = buildGroups(cfg.Groups, root, 0)
	r := lipgloss.NewRenderer(os.Stderr)
	m := &Model{
		root: root,
		styles: styles{
			title:  r.NewStyle().Bold(true),
			cursor: r.NewStyle().Reverse(true),
			group:  r.NewStyle().Bold(true).Foreground(lipgloss.Color("4")),
			dim:    r.NewStyle().Faint(true),
			help:   r.NewStyle().Faint(true),
		},
	}
	m.refresh()
	return m
}

// Selected returns the checked items in config order.
func (m *Model) Selected() []*config.Item {
	var out []*config.Item
	var walk func(*node)
	walk = func(n *node) {
		if !n.isGroup() && n.checked {
			out = append(out, n.item)
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(m.root)
	return out
}

func (m *Model) refresh() {
	m.rows = m.rows[:0]
	var walk func(*node)
	walk = func(n *node) {
		for _, c := range n.children {
			m.rows = append(m.rows, c)
			if c.expanded {
				walk(c)
			}
		}
	}
	walk(m.root)
	m.cursor = max(0, min(m.cursor, len(m.rows)-1))
}

func (m *Model) moveTo(target *node) {
	for i, r := range m.rows {
		if r == target {
			m.cursor = i
			return
		}
	}
}

func (m *Model) listHeight() int { return max(1, m.height-4) }

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		if len(m.rows) == 0 {
			break
		}
		cur := m.rows[m.cursor]
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.aborted = true
			return m, tea.Quit
		case "enter":
			m.done = true
			return m, tea.Quit
		case "up", "k":
			m.cursor--
		case "down", "j":
			m.cursor++
		case "pgup":
			m.cursor -= m.listHeight()
		case "pgdown":
			m.cursor += m.listHeight()
		case "home", "g":
			m.cursor = 0
		case "end", "G":
			m.cursor = len(m.rows) - 1
		case " ", "x":
			if cur.isGroup() {
				ch, t := cur.count()
				cur.setChecked(ch < t)
			} else {
				cur.checked = !cur.checked
			}
		case "a":
			ch, t := m.root.count()
			m.root.setChecked(ch < t)
		case "right", "l":
			if cur.isGroup() {
				if !cur.expanded {
					cur.expanded = true
					m.refresh()
				} else if len(cur.children) > 0 {
					m.cursor++
				}
			}
		case "left", "h":
			if cur.isGroup() && cur.expanded {
				cur.expanded = false
				m.refresh()
			} else if cur.parent != m.root {
				m.moveTo(cur.parent)
			}
		}
		m.cursor = max(0, min(m.cursor, len(m.rows)-1))
	}

	// Keep the cursor inside the visible window.
	h := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	} else if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	m.offset = max(0, min(m.offset, len(m.rows)-h))
	return m, nil
}

func (m *Model) View() string {
	if m.done || m.aborted {
		return ""
	}
	var b strings.Builder
	ch, t := m.root.count()
	b.WriteString(m.styles.title.Render(fmt.Sprintf("Arch post-install · %d/%d selected", ch, t)))
	b.WriteString("\n\n")

	end := min(len(m.rows), m.offset+m.listHeight())
	for i := m.offset; i < end; i++ {
		line := m.renderRow(m.rows[i])
		if m.width > 0 {
			line = ansi.Truncate(line, m.width, "…")
		}
		if i == m.cursor {
			line = m.styles.cursor.Render(ansi.Strip(line))
		}
		b.WriteString(line + "\n")
	}
	for i := end - m.offset; i < m.listHeight(); i++ {
		b.WriteString("\n")
	}

	help := "↑↓ move · space toggle · a all · →← open/close · enter done · q quit"
	if m.width > 0 {
		help = ansi.Truncate(help, m.width, "…")
	}
	b.WriteString("\n" + m.styles.help.Render(help))
	return b.String()
}

func (m *Model) renderRow(n *node) string {
	indent := strings.Repeat("  ", n.depth)
	if n.isGroup() {
		arrow := "▸"
		if n.expanded {
			arrow = "▾"
		}
		ch, t := n.count()
		box := "[ ]"
		switch {
		case t > 0 && ch == t:
			box = "[x]"
		case ch > 0:
			box = "[~]"
		}
		return fmt.Sprintf("%s%s %s %s %s", indent, arrow, box, m.styles.group.Render(n.name), m.styles.dim.Render(fmt.Sprintf("%d/%d", ch, t)))
	}
	box := "[ ]"
	if n.checked {
		box = "[x]"
	}
	line := fmt.Sprintf("%s  %s %s", indent, box, n.name)
	if hint := itemHint(n.item); hint != "" {
		line += "  " + m.styles.dim.Render(hint)
	}
	return line
}

// itemHint shows what an item does when its name alone does not say it.
func itemHint(it *config.Item) string {
	var parts []string
	if !(len(it.Packages) == 1 && it.Packages[0] == it.Name) && len(it.Packages) > 0 {
		parts = append(parts, strings.Join(it.Packages, " "))
	}
	if it.Run != "" {
		parts = append(parts, "+ script")
	}
	return strings.Join(parts, " ")
}

// Run shows the picker on the terminal and returns the checked items.
// It draws to stderr and reads the tty, so stdout stays free for the script.
func Run(cfg *config.Config) ([]*config.Item, error) {
	p := tea.NewProgram(New(cfg), tea.WithAltScreen(), tea.WithOutput(os.Stderr), tea.WithInputTTY())
	res, err := p.Run()
	if err != nil {
		return nil, err
	}
	m := res.(*Model)
	if m.aborted {
		return nil, ErrAborted
	}
	return m.Selected(), nil
}
