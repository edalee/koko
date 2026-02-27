package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func renderAt(width, height int) (raw string, stripped string, m Model) {
	m = New()
	model, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m = model.(Model)

	view := m.View()
	raw = view.Content
	stripped = stripANSI(raw)
	return
}

func TestLayoutDimensions(t *testing.T) {
	sizes := []struct{ w, h int }{
		{120, 40},
		{80, 24},
		{160, 50},
	}

	for _, sz := range sizes {
		t.Run(fmt.Sprintf("%dx%d", sz.w, sz.h), func(t *testing.T) {
			raw, stripped, _ := renderAt(sz.w, sz.h)

			measuredW := lipgloss.Width(raw)
			measuredH := lipgloss.Height(raw)

			t.Logf("Expected: %dx%d, Measured: %dx%d", sz.w, sz.h, measuredW, measuredH)

			if measuredW != sz.w {
				t.Errorf("Width = %d, want %d", measuredW, sz.w)
			}
			if measuredH != sz.h {
				t.Errorf("Height = %d, want %d", measuredH, sz.h)
			}

			// Check each line width
			lines := strings.Split(stripped, "\n")
			for i, line := range lines {
				lineW := len([]rune(line))
				if lineW > sz.w {
					t.Errorf("Line %d overflows: %d chars (max %d): %q", i, lineW, sz.w, line)
				}
			}
		})
	}
}

func TestLayoutVisual(t *testing.T) {
	_, stripped, _ := renderAt(100, 30)

	fmt.Println("=== LAYOUT 100x30 ===")
	lines := strings.Split(stripped, "\n")
	for i, line := range lines {
		fmt.Printf("%2d: |%s|\n", i, line)
	}
	fmt.Printf("=== %d lines ===\n", len(lines))
}

func TestLayoutSidebarHidden(t *testing.T) {
	m := New()
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = model.(Model)

	// Toggle sidebar off
	model, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: '\\', Mod: tea.ModCtrl}))
	m = model.(Model)

	view := m.View()
	raw := view.Content
	stripped := stripANSI(raw)

	fmt.Println("=== SIDEBAR HIDDEN 100x30 ===")
	lines := strings.Split(stripped, "\n")
	for i, line := range lines {
		fmt.Printf("%2d: |%s|\n", i, line)
	}

	measuredW := lipgloss.Width(raw)
	if measuredW != 100 {
		t.Errorf("Width with sidebar hidden = %d, want 100", measuredW)
	}
}
