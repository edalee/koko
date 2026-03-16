# Plan 017: Terminal Rendering Fixes

## Problem

Terminal panes suffer from flicker and rendering glitches. The same issue exists in [attn](https://github.com/victorarias/attn) (Tauri + xterm.js + WebGL), confirming the root cause is **xterm.js WebGL rendering inside macOS WKWebView** — not Koko-specific.

macOS WKWebView compositing with WebGL canvases has known issues: the compositor struggles when repainting glass/blur layers behind the canvas, and the GPU scheduler can deprioritize the WebView canvas during rapid repaints.

## Current Implementation

`frontend/src/components/TerminalPane.tsx`:
- xterm.js v5 + WebGL addon + FitAddon
- PTY output arrives as base64-encoded Wails events, decoded and written individually
- WebGL context loss disposes addon but never recreates it (falls back to slow DOM renderer)
- Inactive tabs use `display: none` which destroys layout and can trigger context loss
- ResizeObserver fires for all terminals including hidden ones
- 50ms resize debounce

## Proposed Fixes (ordered by impact)

### 1. Batch PTY writes
**Impact: High** — biggest visual improvement

Currently every PTY chunk triggers `term.write()`. During fast output (claude streaming), this means dozens of writes/renders per second.

Fix: accumulate chunks in an array, flush on `requestAnimationFrame`:

```typescript
const pendingWrites: Uint8Array[] = [];
let rafId: number | null = null;

const cleanupData = EventsOn(`pty:data:${sessionId}`, (encoded: string) => {
  pendingWrites.push(Uint8Array.from(atob(encoded), (c) => c.charCodeAt(0)));
  if (!rafId) {
    rafId = requestAnimationFrame(() => {
      rafId = null;
      for (const chunk of pendingWrites) {
        term.write(chunk);
      }
      pendingWrites.length = 0;
    });
  }
});
```

### 2. Use `visibility: hidden` instead of `display: none`
**Impact: Medium** — prevents WebGL context loss on tab switch

`display: none` removes the element from layout entirely. WebGL contexts can be lost. `visibility: hidden` + `position: absolute` keeps the canvas alive and the GPU context warm.

```tsx
<div
  ref={containerRef}
  className="h-full w-full p-1"
  style={{
    visibility: active ? "visible" : "hidden",
    position: active ? "relative" : "absolute",
    top: 0, left: 0, right: 0, bottom: 0,
  }}
/>
```

### 3. Recover WebGL on context loss
**Impact: Medium** — prevents permanent fallback to slow DOM renderer

Currently (line 69): `webgl.onContextLoss(() => webgl.dispose())` — disposed and never recreated.

Fix: recreate after a delay:

```typescript
const loadWebGL = () => {
  try {
    const webgl = new WebglAddon();
    webgl.onContextLoss(() => {
      webgl.dispose();
      setTimeout(loadWebGL, 1000);
    });
    term.loadAddon(webgl);
  } catch {
    // DOM renderer fallback
  }
};
loadWebGL();
```

### 4. Skip resize for inactive terminals
**Impact: Low** — reduces unnecessary work

Guard the ResizeObserver callback:

```typescript
const observer = new ResizeObserver(() => {
  if (!active) return; // skip hidden tabs
  // ... existing debounced resize
});
```

Note: `active` would need to be accessed via ref since the observer is created once.

### 5. Disable WebView transparency (nuclear option)
**Impact: Unknown** — may eliminate flicker but breaks glassmorphism

In `main.go`, `WebviewIsTransparent: true` forces extra compositing for the glassmorphism effect. Setting it to `false` removes the transparency compositing overhead but breaks the glass aesthetic.

Worth testing to confirm the root cause. Could lead to a design compromise: opaque terminal area with glass only on sidebars/overlays.

## Reference

attn's approach (more sophisticated but same flicker):
- Separate PTY worker processes with Unix socket IPC
- Sequence-numbered chunks with monotonic ordering
- UTF-8 boundary safety (`findSafeBoundary()`)
- Pending attach queue for ordered replay
- Y-axis resize immediate, X-axis debounced 100ms
- IntersectionObserver for visibility-based rendering

## Verification

1. Open app, create 3+ sessions with active claude output
2. Switch rapidly between tabs — no flash of blank content
3. Resize window during streaming output — no visible stutter
4. Leave session idle 5+ minutes, return — WebGL still active (no DOM fallback)
5. Compare with `WebviewIsTransparent: false` to measure compositing impact
