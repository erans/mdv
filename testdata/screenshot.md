# mdv

A terminal markdown viewer that renders like the desktop it runs on.

## Theme

Colors come from the active Omarchy theme, so `mdv` matches your bar,
your terminal and your editor:

| Role | Source | Used for |
| --- | --- | --- |
| `accent` | `colors.toml` | headings, links, bullets, scrollbar thumb |
| `muted` | `colors.toml` | borders, quotes, footer, scrollbar track |
| `selection` | `colors.toml` | inline code chips |
| six hues | `colors.toml` | syntax highlighting |

```go
func main() {
    pal := resolvePalette("") // theme file, then terminal, then built-in
    run(newViewer("README.md", src, pal))
}
```

- **tables** that fit the pane instead of overflowing it
- **colors** taken from the desktop's own theme, not a hardcoded palette
- **links** you can click, a **scrollbar** you can drag
- **mermaid** flowcharts drawn as box art, inline

## Viewer

| Input | Does |
| --- | --- |
| wheel, `j`/`k` | scroll a line, the wheel moves three |
| click or drag the scrollbar | jump, or scrub through the document |
| click a link | opens it with `xdg-open` |
| `g`/`G`, `Home`/`End` | top, bottom |

> Clicking a link works even though the terminal hands the app its clicks:
> the viewer recovers every link's columns from the rendered output.
