# mdv

Terminal markdown viewer: tables, mermaid diagrams as box art, images where the
terminal can paint them, colored by the desktop's theme.

```
mdv README.md          # render a file
mdv -w 100 README.md   # fixed width
cat x.md | mdv         # stdin
mdv -palette           # show the resolved palette
```

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
- [ ] mermaid diagrams (all six types) as box art
- [ ] images via kitty graphics / iTerm2 / sixel, halfblock fallback

Known gaps: glamour does not expose table border colors, so borders keep the
terminal's default foreground; link URLs are printed after the link text.
