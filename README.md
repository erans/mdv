# mdv

Terminal markdown viewer: tables, mermaid diagrams as box art, images where the
terminal can paint them, colored by the desktop's theme.

```
mdv README.md          # open the viewer
mdv -w 100 README.md   # open it at a fixed width
cat x.md | mdv         # stdin
mdv -plain README.md   # print once, no viewer (also what pipes get)
mdv -palette           # show the resolved palette
```

## Viewer

Full screen, the last column is the scrollbar: the thumb shows how far you
are, and the footer shows the percentage and current line.

| Input | Does |
|---|---|
| wheel, `j`/`k`, `↑`/`↓` | scroll a line (wheel moves three) |
| click / drag the scrollbar | jump, or scrub |
| click a link | opens it with `xdg-open` (`open` on macOS) |
| `d`/`u`, `space`, `PgUp`/`PgDn` | half a page |
| `g`/`G`, `Home`/`End` | top, bottom |
| `q`, `Esc`, `Ctrl+C` | quit |

Clicking a link inside the viewer works even though the terminal hands its
clicks to the app: the renderer keeps its OSC 8 hyperlinks, and the viewer
recovers each link's columns from that output to hit-test the click.

## Theme

Colors come from the first source that answers:

1. `-theme FILE`, or the active Omarchy theme
   (`~/.local/state/omarchy/current/theme/colors.toml`, then Omarchy 2's
   `~/.config/omarchy/current/theme/colors.toml`).
2. The terminal itself: OSC 10/11 and the ANSI hues in one bounded query.
3. A built-in dark palette.

The theme file wins over the terminal on purpose: over SSH the local terminal is
only a window onto someone else's desktop, so the remote theme decides the
colors — and whether the result is light or dark.

Everything renders against twelve palette roles (accent, muted, selection,
foreground, and the six hues), never against hex literals.

## Status

- [x] markdown, tables, syntax highlighting, hyperlinks
- [x] viewer: scrolling, scrollbar with progress, wheel, scrollbar scrub, clickable links
- [ ] mermaid diagrams (all six types) as box art
- [ ] images via kitty graphics / iTerm2 / sixel, halfblock fallback

Known gaps: glamour does not expose table border colors, so borders keep the
terminal's default foreground; link URLs are printed after the link text.
