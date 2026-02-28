package terminal

import tea "charm.land/bubbletea/v2"

// keyToBytes converts a bubbletea KeyPressMsg to the raw byte sequence
// expected by a PTY. This mirrors the encoding logic from x/vt's SendKey
// but writes to a string instead of a pipe.
//
// Application cursor mode and application keypad mode are not queried —
// we always send normal mode sequences.
func keyToBytes(key tea.KeyPressMsg) string {
	var seq string

	// Handle Alt modifier: prepend ESC, then strip Alt for matching.
	if key.Mod&tea.ModAlt != 0 {
		seq = "\x1b"
		key.Mod &^= tea.ModAlt
	}

	// Clear BaseCode and ShiftedCode for clean comparison.
	key.BaseCode = 0
	key.ShiftedCode = 0

	switch {
	// Ctrl+letter → C0 control codes
	case key.Mod == tea.ModCtrl && key.Code >= 'a' && key.Code <= 'z':
		seq += string(rune(key.Code - 'a' + 1))

	// Ctrl+special
	case key.Mod == tea.ModCtrl && key.Code == tea.KeySpace:
		seq += "\x00"
	case key.Mod == tea.ModCtrl && key.Code == '[':
		seq += "\x1b"
	case key.Mod == tea.ModCtrl && key.Code == '\\':
		seq += "\x1c"
	case key.Mod == tea.ModCtrl && key.Code == ']':
		seq += "\x1d"
	case key.Mod == tea.ModCtrl && key.Code == '^':
		seq += "\x1e"
	case key.Mod == tea.ModCtrl && key.Code == '_':
		seq += "\x1f"

	// Special keys (no modifiers required)
	case key.Code == tea.KeyEnter && key.Mod == 0:
		seq += "\r"
	case key.Code == tea.KeyTab && key.Mod == 0:
		seq += "\t"
	case key.Code == tea.KeyBackspace && key.Mod == 0:
		seq += "\x7f"
	case key.Code == tea.KeyEscape && key.Mod == 0:
		seq += "\x1b"

	// Shift+Tab
	case key.Code == tea.KeyTab && key.Mod == tea.ModShift:
		seq += "\x1b[Z"

	// Arrow keys (normal mode)
	case key.Code == tea.KeyUp && key.Mod == 0:
		seq += "\x1b[A"
	case key.Code == tea.KeyDown && key.Mod == 0:
		seq += "\x1b[B"
	case key.Code == tea.KeyRight && key.Mod == 0:
		seq += "\x1b[C"
	case key.Code == tea.KeyLeft && key.Mod == 0:
		seq += "\x1b[D"

	// Navigation keys
	case key.Code == tea.KeyInsert && key.Mod == 0:
		seq += "\x1b[2~"
	case key.Code == tea.KeyDelete && key.Mod == 0:
		seq += "\x1b[3~"
	case key.Code == tea.KeyHome && key.Mod == 0:
		seq += "\x1b[H"
	case key.Code == tea.KeyEnd && key.Mod == 0:
		seq += "\x1b[F"
	case key.Code == tea.KeyPgUp && key.Mod == 0:
		seq += "\x1b[5~"
	case key.Code == tea.KeyPgDown && key.Mod == 0:
		seq += "\x1b[6~"

	// Function keys
	case key.Code == tea.KeyF1 && key.Mod == 0:
		seq += "\x1bOP"
	case key.Code == tea.KeyF2 && key.Mod == 0:
		seq += "\x1bOQ"
	case key.Code == tea.KeyF3 && key.Mod == 0:
		seq += "\x1bOR"
	case key.Code == tea.KeyF4 && key.Mod == 0:
		seq += "\x1bOS"
	case key.Code == tea.KeyF5 && key.Mod == 0:
		seq += "\x1b[15~"
	case key.Code == tea.KeyF6 && key.Mod == 0:
		seq += "\x1b[17~"
	case key.Code == tea.KeyF7 && key.Mod == 0:
		seq += "\x1b[18~"
	case key.Code == tea.KeyF8 && key.Mod == 0:
		seq += "\x1b[19~"
	case key.Code == tea.KeyF9 && key.Mod == 0:
		seq += "\x1b[20~"
	case key.Code == tea.KeyF10 && key.Mod == 0:
		seq += "\x1b[21~"
	case key.Code == tea.KeyF11 && key.Mod == 0:
		seq += "\x1b[23~"
	case key.Code == tea.KeyF12 && key.Mod == 0:
		seq += "\x1b[24~"

	// Keypad keys (normal mode — no application keypad awareness)
	case key.Code == tea.KeyKpEnter && key.Mod == 0:
		seq += "\r"
	case key.Code == tea.KeyKpEqual && key.Mod == 0:
		seq += "="
	case key.Code == tea.KeyKpMultiply && key.Mod == 0:
		seq += "*"
	case key.Code == tea.KeyKpPlus && key.Mod == 0:
		seq += "+"
	case key.Code == tea.KeyKpComma && key.Mod == 0:
		seq += ","
	case key.Code == tea.KeyKpMinus && key.Mod == 0:
		seq += "-"
	case key.Code == tea.KeyKpDecimal && key.Mod == 0:
		seq += "."
	case key.Code == tea.KeyKpDivide && key.Mod == 0:
		seq += "/"
	case key.Code >= tea.KeyKp0 && key.Code <= tea.KeyKp9 && key.Mod == 0:
		seq += string(rune('0' + (key.Code - tea.KeyKp0)))

	// Printable characters
	default:
		if key.Mod != 0 {
			return seq // Unknown modifier combo — don't send garbage
		}
		if key.Text != "" {
			seq += key.Text
		} else if key.Code > 0 {
			seq += string(key.Code)
		}
	}

	return seq
}
