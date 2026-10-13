package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FelixDes/arch_post_install/internal/config"
)

func testModel(t *testing.T) *Model {
	t.Helper()
	cfg, err := config.Parse([]byte(`
groups:
  - name: A
    groups:
      - name: Sub
        items: [s1]
    items:
      - a1
      - name: a2
        enabled: false
  - name: B
    items: [b1]
`))
	if err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return m
}

func press(m *Model, keys ...string) {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "left":
			msg = tea.KeyMsg{Type: tea.KeyLeft}
		case "right":
			msg = tea.KeyMsg{Type: tea.KeyRight}
		case "space":
			msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		m.Update(msg)
	}
}

func names(items []*config.Item) []string {
	var out []string
	for _, it := range items {
		out = append(out, it.Name)
	}
	return out
}

func TestDefaultsAndOrder(t *testing.T) {
	m := testModel(t)
	if got := names(m.Selected()); len(got) != 3 || got[0] != "s1" || got[1] != "a1" || got[2] != "b1" {
		t.Errorf("selected = %v", got)
	}
	if len(m.rows) != 2 {
		t.Errorf("top-level groups should start collapsed, rows = %d", len(m.rows))
	}
}

func TestNavigateAndToggle(t *testing.T) {
	m := testModel(t)
	press(m, "right") // expand A
	if len(m.rows) != 5 {
		t.Fatalf("rows after expand = %d", len(m.rows))
	}
	press(m, "right", "down", "down", "space") // into A, past Sub and a1, toggle a2 on
	if m.rows[m.cursor].name != "a2" {
		t.Fatalf("cursor on %q", m.rows[m.cursor].name)
	}
	if got := names(m.Selected()); len(got) != 4 {
		t.Errorf("selected = %v", got)
	}
	press(m, "left") // jump to parent A
	if m.rows[m.cursor].name != "A" {
		t.Fatalf("cursor on %q", m.rows[m.cursor].name)
	}
	press(m, "space") // A fully checked -> uncheck all of it
	if got := names(m.Selected()); len(got) != 1 || got[0] != "b1" {
		t.Errorf("selected = %v", got)
	}
	press(m, "left") // collapse A
	if len(m.rows) != 2 {
		t.Errorf("rows after collapse = %d", len(m.rows))
	}
}

func TestToggleAll(t *testing.T) {
	m := testModel(t)
	press(m, "a")
	if got := names(m.Selected()); len(got) != 4 {
		t.Errorf("a with partial selection should check all, got %v", got)
	}
	press(m, "a")
	if got := m.Selected(); len(got) != 0 {
		t.Errorf("a with full selection should uncheck all, got %v", names(got))
	}
}

func TestScrollKeepsCursorVisible(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 6}) // list height 2
	press(m, "right", "down", "down", "down")
	if m.cursor < m.offset || m.cursor >= m.offset+m.listHeight() {
		t.Errorf("cursor %d outside window [%d, %d)", m.cursor, m.offset, m.offset+m.listHeight())
	}
	press(m, "g")
	if m.offset != 0 {
		t.Errorf("offset after home = %d", m.offset)
	}
}
