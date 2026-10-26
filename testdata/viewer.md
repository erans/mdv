# mdv viewer

The viewer does three things beyond scrolling: it searches, it shows a table of
contents, and it opens the links you click.

## Search

Press `/`, type, and the footer counts the matches as you go. `enter` jumps to
the first one, `n` and `N` walk forward and back, `esc` clears the search. Every
match on screen is drawn in reverse video.

## Table of contents

Press `t` for the pane on the right: headings, indented by level, with the
section you are reading marked in the accent colour. `j`/`k` move, `enter`
jumps, clicking a heading jumps too, `esc` closes it. The document re-wraps to
the narrower width, so nothing is cut off.

### Nested headings

Subsections are indented under their parent, which makes a long README feel like
a map instead of a scroll.

### Another subsection

Two subsections here so the pane has something to show.

## Links and scrolling

Click a link to open it, drag the scrollbar to scrub, or use the wheel. The
percentage and line count in the footer tell you where you are.

## Images

A paragraph that is one image is painted where it stands.
