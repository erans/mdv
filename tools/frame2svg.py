#!/usr/bin/env python3
"""Turn one `mdv -frame` capture into an SVG screenshot.

    mdv -frame -w 92 -h 26 README.md | tools/frame2svg.py > docs/screenshot.svg
"""
import re
import sys
import unicodedata
import xml.sax.saxutils as esc

FONT = 14
CW = FONT * 0.602  # monospace advance
LH = FONT * 1.34
PAD = 16
BG = "#11111b"
FG = "#cdd6f4"

SGR = re.compile(r"\x1b\[([0-9;:]*)m")
OSC = re.compile(r"\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)")


def runs(line):
    """Split a line into (text, color, background, bold, underline) runs.

    SGR colours arrive in both forms: `38;2;r;g;b` (semicolon separated) and
    `38:2:r:g:b` (colon separated, what recent terminal libraries emit).
    """
    out, fg, bg, bold, under, i = [], None, None, False, False, 0
    while i < len(line):
        m = SGR.match(line, i)
        if m:
            params = m.group(1).split(";")
            j = 0
            while j < len(params):
                sub = params[j].split(":")
                n = int(sub[0]) if sub[0].isdigit() else 0
                if n in (38, 48):
                    rgb = None
                    if len(sub) >= 5 and sub[1] == "2":
                        rgb = sub[2:5]
                    elif j + 4 < len(params) and params[j + 1] == "2":
                        rgb = params[j + 2 : j + 5]
                        j += 4
                    if rgb:
                        hexed = "#%02x%02x%02x" % (int(rgb[0]), int(rgb[1]), int(rgb[2]))
                        if n == 38:
                            fg = hexed
                        else:
                            bg = hexed
                elif n == 0:
                    fg = bg = None
                    bold = under = False
                elif n == 1:
                    bold = True
                elif n == 4:
                    under = True
                elif n in (22, 23, 24, 25, 27, 28, 29):
                    bold = under = False
                elif n == 39:
                    fg = None
                elif n == 49:
                    bg = None
                j += 1
            i = m.end()
            continue
        o = OSC.match(line, i)
        if o:
            i = o.end()
            continue
        j = min([x for x in (line.find("\x1b", i), len(line)) if x >= 0])
        j = j if j > i else i + 1
        out.append((line[i:j], fg, bg, bold, under))
        i = j
    return out


def cells(text):
    """Terminal cells in a string: wide characters take two columns."""
    return sum(2 if unicodedata.east_asian_width(c) in "WF" else 1 for c in text)


def main():
    rows = [r for r in sys.stdin.read().replace("\r\n", "\n").split("\n")]
    if rows and rows[-1] == "":
        rows.pop()
    cols = max((cells(OSC.sub("", SGR.sub("", r))) for r in rows), default=80)
    w = int(cols * CW + 2 * PAD)
    h = int(len(rows) * LH + 2 * PAD)

    # Every run carries its own x and textLength, so the frame stays aligned
    # whatever monospace font the viewer of the SVG ends up with.
    body = []
    for n, row in enumerate(rows):
        y = PAD + LH * (n + 0.8)
        col = 0
        for text, fg, bg, bold, under in runs(row):
            if not text:
                continue
            attrs = ""
            if fg:
                attrs += f' fill="{fg}"'
            if bg:
                attrs += f' style="background:{bg}"'
            if bold:
                attrs += ' font-weight="600"'
            if under:
                attrs += ' text-decoration="underline"'
            width = cells(text)
            # Block characters are drawn as rectangles: a glyph would leave
            # gaps between rows instead of forming a solid scrollbar.
            if set(text) <= {"\u2588", "\u2580", "\u2502"}:
                for k, ch in enumerate(text):
                    x = PAD + (col + k) * CW
                    top = y - LH * 0.78
                    if ch == "\u2588":
                        body.append(
                            f'<rect x="{x:.1f}" y="{top:.1f}" width="{CW:.1f}" '
                            f'height="{LH:.1f}" fill="{fg or FG}"/>'
                        )
                    elif ch == "\u2580":
                        # Half block: the upper pixel in the foreground, the
                        # lower one in the background.
                        body.append(
                            f'<rect x="{x:.1f}" y="{top:.1f}" width="{CW:.1f}" '
                            f'height="{LH / 2:.1f}" fill="{fg or FG}"/>'
                        )
                        if bg:
                            body.append(
                                f'<rect x="{x:.1f}" y="{top + LH / 2:.1f}" width="{CW:.1f}" '
                                f'height="{LH / 2:.1f}" fill="{bg}"/>'
                            )
                    else:
                        body.append(
                            f'<rect x="{x + CW / 2 - 0.7:.1f}" y="{y - LH * 0.78:.1f}" '
                            f'width="1.4" height="{LH:.1f}" fill="{fg or FG}"/>'
                        )
                col += width
                continue
            body.append(
                f'<text x="{PAD + col * CW:.1f}" y="{y:.1f}" '
                f'textLength="{width * CW:.1f}" lengthAdjust="spacingAndGlyphs" '
                f'xml:space="preserve"{attrs}>{esc.escape(text)}</text>'
            )
            col += width

    sys.stdout.write(
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" '
        f'viewBox="0 0 {w} {h}" font-family="ui-monospace,SFMono-Regular,Menlo,Consolas,monospace" '
        f'font-size="{FONT}" fill="{FG}">\n'
        f'<rect width="{w}" height="{h}" rx="10" fill="{BG}"/>\n'
        + "\n".join(body)
        + "\n</svg>\n"
    )


if __name__ == "__main__":
    main()
