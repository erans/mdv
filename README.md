# mdv

**A markdown viewer that looks like it belongs on your desktop.**

![mdv rendering a README](docs/screenshot.svg)

The screenshots in this file are renders of real viewer frames, not mockups:
`mdv -frame` prints one frame, `tools/frame2svg.py` turns it into an SVG, and
`tools/screenshots.sh` regenerates all of them.

Your terminal already has a theme. Your bar, your editor and your file manager
already match it. Then you `cat README.md` and get a wall of plain text with a
table that wraps into nonsense.

`mdv` renders the README properly — headings, tables that fit the pane,
highlighted code, mermaid diagrams drawn as box art, clickable links, a
scrollbar you can drag — **in the colors of the theme you are already running**.

## Why this one

- **It reads your theme, it does not ship one.** On Omarchy that means the exact
  palette your desktop is wearing: `accent` on headings and links, `muted` on
  borders, `selection` behind inline code. Change the theme, `mdv` changes with
  it. No config file, no `--style` flag to remember.
- **Diagrams are drawn, not printed.** Every mermaid type `mdv` knows —
  flowchart, sequence, state, class, entity relationship — becomes box art in
  the flow of the document, with its shapes, labels, dashed edges and titled
  frames intact.
- **Tables that wrap, never clip.** Columns shrink down to their longest word
  and cells wrap; a table too narrow to lay out is printed as source instead of
  losing text. Borders follow the theme, headers are bold, `code` keeps its
  colour.
- **Links you can actually click.** Terminals hand clicks to the app, so a plain
  hyperlink would never fire inside a full-screen viewer. `mdv` recovers the
  link spans from the rendered output and opens the one you clicked.
- **Pictures stay pictures.** A README image is painted in place — with the
  kitty graphics protocol, iTerm2's inline images, sixel, or half blocks where
  none of those exist. Remote URLs are never fetched; a file that cannot be
  painted falls back to its alt text.
- **A scrollbar with a position, not a guess.** Thumb on the right edge, and a
  footer that says `42%` and `118/254` so you know where you are.

## Install

```sh
go install github.com/erans/mdv@latest    # needs Go 1.26+
# or from a clone
go build -o mdv .
```

## Use

```sh
mdv README.md            # open the viewer
mdv -w 100 README.md     # fixed width
cat x.md | mdv           # stdin
mdv -plain README.md     # print once, no viewer (what pipes get anyway)
mdv -palette             # show the resolved palette and exit
mdv -theme my.toml x.md  # use a specific colors.toml
```

| Flag | Does |
| --- | --- |
| `-w` | wrap width in columns (default: terminal width) |
| `-theme FILE` | a `colors.toml` to use instead of the desktop theme |
| `-plain` | print once instead of opening the viewer; automatic when not a TTY |
| `-palette` | print the resolved palette and exit |
| `-frame`, `-h` | print one viewer frame of `-h` rows and exit (screenshots, tests) |
| `-images MODE` | `auto` (default), `kitty`, `iterm2`, `sixel`, `halfblock`, `off` |

| Input | Does |
| --- | --- |
| wheel, `j`/`k`, `↑`/`↓` | scroll a line (the wheel moves three) |
| click or drag the scrollbar | jump, or scrub through the document |
| click a link | open it with `xdg-open` (`open` on macOS) |
| `d`/`u`, `space`, `PgUp`/`PgDn` | half a page |
| `g`/`G`, `Home`/`End` | top, bottom |
| `q`, `Esc`, `Ctrl+C` | quit |

## Theme

Colors come from the first source that answers:

1. `-theme FILE`, or the active Omarchy theme —
   `~/.local/state/omarchy/current/theme/colors.toml`, then Omarchy 2's
   `~/.config/omarchy/current/theme/colors.toml`.
2. The terminal itself: OSC 10/11 and the ANSI hues, in one bounded query.
3. A built-in dark palette.

**The theme file wins over the terminal on purpose.** Over SSH your local
terminal is only a window onto someone else's desktop, so the remote theme
decides the colors — and whether the result is light or dark.

Everything renders against twelve palette roles (`accent`, `muted`, `selection`,
`foreground` and six hues). `palette.go` is the only file that holds hex
values; the renderers speak role names.

## Mermaid

Fences are drawn as box art, inline, in the flow of the document:

![mdv drawing mermaid graphs and flowcharts](docs/flowchart.svg)

Every type `mdv` knows is drawn: `graph`/`flowchart`, `sequenceDiagram`,
`stateDiagram-v2`, `classDiagram` and `erDiagram`.

For flowcharts: `LR`, `RL`, `TD` and `TB`, square `[]`, rounded `()` and
decision `{}` nodes, `-->` `---` `-.->` `==>` edges with `|labels|` or
`-- labels -->`, chains (`A --> B --> C`), fan-out (`A --> B & C`) and
`subgraph name[Title]` frames, nested ones included.

```mermaid
graph LR
  A[README.md] --> B{mermaid fence?}
  B -->|yes| C[draw box art]
  B -->|no| D[render markdown]
  C --> E[your terminal]
  D --> E
```

Every edge reserves its own lane in the gap it crosses, so two arrows between
the same ranks never overlap, and a long edge swings around the boxes it skips
instead of cutting through them. A frame is sized around its members after the
layout, and a node that would fall inside somebody else's frame is pushed out
of it rather than enclosed by mistake. A diagram too wide for the pane, or a
type the renderer does not know, is printed as source — never clipped.

### Sequence, state, class and entities

| Sequence and state | Class and entities |
| --- | --- |
| [![mdv drawing a mermaid sequence diagram and a state diagram](docs/sequence.svg)](docs/sequence.svg) | [![mdv drawing a mermaid class diagram and an entity relationship diagram](docs/models.svg)](docs/models.svg) |

<details>
<summary>The four sources behind those screenshots</summary>

```mermaid
sequenceDiagram
  participant A as Browser
  participant B as Server
  A->>B: GET /readme
  B-->>A: 200 markdown
```

```mermaid
stateDiagram-v2
  [*] --> Idle
  Idle --> Parsing : fence
  Parsing --> Drawing
  Parsing --> Failed : bad syntax
  Drawing --> Idle : done
```

```mermaid
classDiagram
  class Renderer {
    +String name
    +render(src)
  }
  Renderer <|-- Flow
  Renderer <|-- Sequence
  Flow --> Box : draws
```

```mermaid
erDiagram
  CUSTOMER ||--o{ ORDER : places
  ORDER ||--|{ LINE_ITEM : contains
  CUSTOMER {
    string id PK
    string email
  }
```

</details>

- **Sequence**: participant boxes and lifelines, `->>` solid and `-->>` dashed
  messages, self messages, labels centred between the lifelines.
- **State**: `[*]` start and end dots, `A --> B : label` transitions, cycles
  ranked by breaking back edges first so the diagram does not run downwards
  forever.
- **Class**: compartments for members, inheritance and association arrows
  pointing at the class the mermaid arrowhead points at.
- **Entities**: attribute rows inside the entity box, relationship labels from
  `A ||--o{ B : label`, whatever cardinality markers mermaid uses.

## Images

![mdv painting a README image with half blocks](docs/images.svg)

A paragraph that is nothing but `![alt](file.png)` becomes the picture itself,
at the width of the pane:

- **kitty** — the graphics protocol, by terminal class name. Works in kitty,
  Ghostty, WezTerm and Warp.
- **iTerm2** — OSC 1337 inline images.
- **sixel** — for terminals that speak it, encoded here rather than shelled out.
- **half blocks** — `▀` with a foreground and background colour per cell. Not a
  protocol at all, so it works everywhere and scrolls with the text.

`-images` picks one; `auto` uses the protocol the terminal reports, and half
blocks inside the viewer, where scrolling redraws every line and a one-shot
graphics escape would be painted twice. Piping gets the protocol.

## How it works

- Markdown → ANSI is [glamour](https://github.com/charmbracelet/glamour), so
  text, code and syntax highlighting are not reinvented here. Tables are cut out
  first and laid out by `table.go`, because a cell must wrap rather than be
  clipped when the table lands on the wrap width, and because a border should
  take the theme's colour.
- Mermaid fences are cut out of the document before glamour sees them, drawn by
  the `mermaid` package, and spliced back in.
- The viewer is [bubbletea](https://github.com/charmbracelet/bubbletea), with a
  hand-drawn scrollbar and footer.
- Link spans are recovered by walking the rendered lines and counting visible
  columns while skipping SGR and OSC sequences, so a click maps to exactly the
  characters the terminal drew.
- `-frame` prints one viewer frame, so a screenshot is just a frame piped
  through `tools/frame2svg.py`. `tools/screenshots.sh` does that for every image
  in `docs/`:

  ```sh
  ./tools/screenshots.sh
  ```

## Known limits

- A decision node (`{}`) is drawn as a `╱──╲` frame, not a true diamond.
- Sequence diagrams ignore `alt`/`loop`/`note` blocks; only messages are drawn.
- Class relation arrowheads are one shape; UML's hollow triangles and diamonds
  are not distinguished.
- `direction` inside a `subgraph` is ignored; frames follow the diagram.
- Fences inside a list item or blockquote stay a code block.
- An edge label can end up touching a box corner when the gap is tight.
- Only local images are painted: `![](https://…)` stays alt text, and an image
  sharing a paragraph with text is left to the markdown renderer.
- Cell size is assumed (10x20 px) when computing how many rows an image takes,
  so a picture can come out a row taller or shorter than its box needs.

## Status

Done:

- [x] markdown, tables, syntax highlighting, hyperlinks
- [x] viewer: scrolling, scrollbar with progress, wheel, scrollbar scrub, clickable links
- [x] Omarchy palettes, terminal queries, built-in fallback
- [x] mermaid flowcharts as box art (LR/RL/TD/TB, shapes, labels, dashed edges)
- [x] `subgraph` frames with titles, nested frames, outsiders kept out
- [x] mermaid sequence, state, class and ER diagrams
- [x] inline images via kitty graphics, iTerm2, sixel and half blocks

Next:

- [x] own table renderer: wrapped cells, themed borders, nothing clipped
- [ ] mermaid `alt`/`loop`/`note` blocks in sequence diagrams

`go test ./...` covers the palette parser, the OSC reply parser, the link
recovery, the scrollbar math, the mermaid layouts, the image protocols and the
frame layout.
