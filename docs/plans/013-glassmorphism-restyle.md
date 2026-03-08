# Plan 013: Glassmorphism Restyle

## Context

Koko's current design uses a plum toolbar (`#4A1A33`), flat VS Code-style dark grays (`#1e1e1e`, `#252526`), and distinct elevation levels. The goal is to restyle toward a premium glassmorphism aesthetic inspired by Warp, Cursor, Linear, and Arc — with a blurry transparent main screen, layered glass cards creating depth, and the existing mint gradient accent preserved.

**Keep:** Mint gradient (`#1FF2AB` → `#24A965`), dark foundation, layout structure
**Kill:** Plum toolbar, distinct surface hex colors, flat card style
**Add:** Mesh gradient background, glass elevation system, transparency-based depth, subtle noise texture

## Research Sources

- **Warp** — semi-transparent white overlays on base + outline borders for UI surfaces
- **Cursor** — ultra-low opacity overlays (6-8%), warm dark base (`#14120b`), barely-visible borders
- **Linear** — surface elevation via lightness, LCH color space, single accent color restraint
- **Arc** — native vibrancy, desktop-tinted sidebar, noise texture overlay
- **Raycast** — frosted glass panels, monochrome base + strategic accent pops
- **Aceternity UI** — glass card recipes, bento grids, dark theme patterns

## New Design System

### Base Background

Replace the flat `#1e1e1e` with a slightly warm dark + mesh gradient orbs that the glass distorts:

```css
--color-base: #0f1117;  /* deep dark blue-black (cooler, richer than #1e1e1e) */
```

Mesh gradient orbs (applied to root/body):
```css
background: var(--color-base);
/* Mint orb bottom-left */
radial-gradient(ellipse at 15% 85%, rgba(31, 242, 171, 0.07), transparent 50%),
/* Blue orb top-right */
radial-gradient(ellipse at 85% 20%, rgba(60, 120, 220, 0.06), transparent 50%),
/* Teal orb center */
radial-gradient(ellipse at 50% 50%, rgba(0, 180, 200, 0.04), transparent 60%);
```

Optional SVG noise overlay at 2-3% opacity for tactile grain.

### Elevation System (Glass Tiers)

Replace distinct hex colors with white-overlay transparency tiers:

| Token | Old Value | New Value | Blur | Use |
|-------|-----------|-----------|------|-----|
| `--color-base` | `#1e1e1e` | `#0f1117` | none | App bg with mesh gradients |
| `--color-surface-1` | — | `rgba(255,255,255, 0.03)` | `blur(20px)` | Sidebar panels |
| `--color-surface-2` | `#252526` | `rgba(255,255,255, 0.06)` | `blur(12px)` | Cards, content areas |
| `--color-surface-3` | `rgba(30,30,30,0.85)` | `rgba(255,255,255, 0.10)` | `blur(16px)` | Modals, overlays |
| `--color-toolbar` | `#4a1a33` | `rgba(255,255,255, 0.04)` | `blur(24px)` | Toolbar (glass, no plum) |

### Border System

All borders become ultra-thin white at low opacity (Linear/Cursor pattern):

| Token | Old Value | New Value |
|-------|-----------|-----------|
| `--color-border` | `rgba(255,255,255, 0.1)` | `rgba(255,255,255, 0.08)` |
| `--color-border-hover` | — | `rgba(255,255,255, 0.14)` |
| `--color-glass-border` | `rgba(255,255,255, 0.08)` | `rgba(255,255,255, 0.10)` |

### Text Opacity Hierarchy (Linear Pattern)

Replace muted-foreground with an opacity-based system:

| Level | Old | New | Use |
|-------|-----|-----|-----|
| Primary | `#cccccc` | `rgba(255,255,255, 0.92)` | Headings, active items |
| Secondary | `#888888` | `rgba(255,255,255, 0.55)` | Body text, labels |
| Tertiary | — | `rgba(255,255,255, 0.35)` | Hints, timestamps, placeholders |
| Quaternary | — | `rgba(255,255,255, 0.20)` | Disabled, decorative |

### Accent (Unchanged)

```
--color-accent: #1ff2ab
--color-accent-dark: #24a965
```

Active states: `border: 1px solid rgba(31, 242, 171, 0.20)` + `box-shadow: 0 0 20px rgba(31, 242, 171, 0.08)`

### Glass Card Recipe

Standard card (used for PR cards, Slack items, mail items, session cards):
```css
.glass-card {
  background: rgba(255, 255, 255, 0.06);
  backdrop-filter: blur(12px) saturate(1.5);
  -webkit-backdrop-filter: blur(12px) saturate(1.5);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

.glass-card:hover {
  background: rgba(255, 255, 255, 0.09);
  border-color: rgba(255, 255, 255, 0.12);
}

.glass-card--active {
  border-color: rgba(31, 242, 171, 0.20);
  box-shadow: 0 0 20px rgba(31, 242, 171, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.04);
}
```

### Shadow System

Minimal — dark backgrounds make shadows subtle. Rely on glass opacity differences:
```css
--shadow-sm: 0 2px 8px rgba(0, 0, 0, 0.2);
--shadow-md: 0 8px 32px rgba(0, 0, 0, 0.25);
--shadow-glow: 0 0 20px rgba(31, 242, 171, 0.08);
```

## Changes Per Component

### globals.css — Theme Token Overhaul
- Replace all color tokens with new system
- Add mesh gradient background to `html, body, #root`
- Add glass utility classes (`.glass-1`, `.glass-2`, `.glass-3`)
- Add noise texture pseudo-element
- Add `backdrop-filter` + `-webkit-backdrop-filter` utilities
- Update animation keyframes if needed

### Toolbar.tsx
- `bg-toolbar` (#4a1a33 plum) → glass toolbar: `rgba(255,255,255, 0.04)` + `blur(24px)`
- Keep drag region, traffic light spacer, logo
- Notification buttons: already use `bg-white/10` active, keep as-is
- Settings button: keep ghost style

### SessionSidebar.tsx
- `bg-base` → glass surface-1: `rgba(255,255,255, 0.03)` + `blur(20px)`
- Search input: `bg-white/5` → `bg-white/[0.04]`, softer border
- New Session button: keep mint gradient border, adjust inner bg to glass
- Session cards: use glass-card pattern with active glow
- Delete button red stays (`#F14D4C`)

### RightSidebar.tsx
- `bg-surface` icon bar → glass surface-1
- `bg-base` module content → glass surface-1
- Module buttons: keep accent gradient active state

### GitHubPanel.tsx, SlackPanel.tsx, MailPanel.tsx
- PR/message cards: `bg-white/5 border border-border` → glass-card class
- SlackPanel avatar: `text-toolbar` (plum) → `text-base` or dark text on mint bg
- MailPanel: keep colored gradients for type badges

### OverlayPage.tsx, NewSessionDialog.tsx
- Already use glass pattern (`--color-glass`, `--color-glass-border`)
- Update to surface-3 tier values
- May increase blur from 40px to keep consistency

### TerminalPane.tsx
- Update xterm.js theme background from `#1e1e1e` to `#0f1117`
- Cursor stays `#1FF2AB`
- Foreground may need adjustment for contrast against new bg

### NotificationBadge.tsx
- No changes needed (uses accent color)

### App.tsx
- `bg-base` on root container — will pick up new base color
- Add mesh gradient background layer (via CSS or inline)

## New CSS Tokens Summary

```css
@theme {
  /* Base */
  --color-base: #0f1117;

  /* Glass surfaces (used with backdrop-filter) */
  --color-surface-1: rgba(255, 255, 255, 0.03);
  --color-surface-2: rgba(255, 255, 255, 0.06);
  --color-surface-3: rgba(255, 255, 255, 0.10);

  /* Toolbar (glass, replaces plum) */
  --color-toolbar: rgba(255, 255, 255, 0.04);

  /* Text (opacity-based hierarchy) */
  --color-foreground: rgba(255, 255, 255, 0.92);
  --color-muted-foreground: rgba(255, 255, 255, 0.55);
  --color-tertiary: rgba(255, 255, 255, 0.35);
  --color-quaternary: rgba(255, 255, 255, 0.20);

  /* Borders */
  --color-border: rgba(255, 255, 255, 0.08);
  --color-glass-border: rgba(255, 255, 255, 0.10);

  /* Accent (unchanged) */
  --color-accent: #1ff2ab;
  --color-accent-dark: #24a965;

  /* Glass overlays */
  --color-glass: rgba(255, 255, 255, 0.10);

  /* Semantic (unchanged) */
  --color-success: oklch(0.65 0.15 155);
  --color-warning: oklch(0.75 0.15 80);
  --color-error: oklch(0.65 0.2 25);

  /* Badges */
  --color-badge-slack: #e01e5a;
  --color-badge-mail: #569cd6;
}
```

## Implementation Order

1. **globals.css** — New tokens, mesh gradient bg, glass utility classes, noise texture
2. **Toolbar.tsx** — Glass toolbar (biggest visual change, validates the look early)
3. **SessionSidebar.tsx** — Glass sidebar + glass session cards
4. **RightSidebar.tsx** — Glass icon bar + content
5. **OverlayPage.tsx + NewSessionDialog.tsx** — Update glass tier values
6. **GitHubPanel.tsx, SlackPanel.tsx, MailPanel.tsx** — Glass cards, fix plum text refs
7. **TerminalPane.tsx** — Update xterm theme colors
8. **App.tsx** — Mesh gradient background layer if not handled in CSS

## Verification

1. `make dev` — app renders with new glass aesthetic
2. Toolbar is translucent glass (no plum)
3. Sidebars have visible glass effect with mesh gradients showing through
4. Cards create depth with layered glass tiers
5. Mint accent pops against neutral glass surfaces
6. Text hierarchy is clear (primary → secondary → tertiary)
7. Terminal cursor and accent elements still use mint
8. Overlays/modals have frosted glass with blur
9. No plum color references remain
10. All existing functionality works (sessions, PRs, overlays)
