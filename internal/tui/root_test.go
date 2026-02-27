package tui

import (
	"flag"
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	snapshotSize   = flag.String("snapshot-size", "100x30", "WIDTHxHEIGHT for snapshot render")
	snapshotHidden = flag.Bool("snapshot-hidden", false, "render with sidebar hidden")
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

func TestSnapshot(t *testing.T) {
	var w, h int
	if _, err := fmt.Sscanf(*snapshotSize, "%dx%d", &w, &h); err != nil {
		t.Fatalf("invalid -snapshot-size %q: %v", *snapshotSize, err)
	}

	m := New()
	model, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m = model.(Model)

	sidebarState := "visible"
	if *snapshotHidden {
		model, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: '\\', Mod: tea.ModCtrl}))
		m = model.(Model)
		sidebarState = "hidden"
	}

	view := m.View()
	raw := view.Content
	stripped := stripANSI(raw)

	measuredW := lipgloss.Width(raw)
	measuredH := lipgloss.Height(raw)

	// Header
	fmt.Printf("\n=== SNAPSHOT %dx%d (sidebar: %s, focus: %s) ===\n", w, h, sidebarState, m.focus)
	fmt.Printf("Measured: %dx%d", measuredW, measuredH)
	if measuredW != w || measuredH != h {
		fmt.Printf(" *** MISMATCH (expected %dx%d) ***", w, h)
	}
	fmt.Println()
	fmt.Println()

	// Render with line numbers and edge markers
	lines := strings.Split(stripped, "\n")
	overflows := 0
	for i, line := range lines {
		lineW := len([]rune(line))
		marker := " "
		if lineW > w {
			marker = "!"
			overflows++
		}
		fmt.Printf("%s%2d: |%s|\n", marker, i, line)
	}

	// Footer
	fmt.Println()
	if overflows > 0 {
		fmt.Printf("!!! %d lines overflow (marked with !)\n", overflows)
	} else {
		fmt.Println("All lines within bounds.")
	}
	fmt.Printf("=== END %dx%d ===\n\n", w, h)

	// Fail test if dimensions don't match
	if measuredW != w {
		t.Errorf("Width = %d, want %d", measuredW, w)
	}
	if measuredH != h {
		t.Errorf("Height = %d, want %d", measuredH, h)
	}
}
