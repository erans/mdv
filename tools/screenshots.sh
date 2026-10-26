#!/bin/sh
# Regenerate every screenshot in docs/ from its testdata document.
#
# The SVGs are renders of real viewer frames: `mdv -frame` prints one frame and
# tools/frame2svg.py turns it into an SVG.
set -eu

cd "$(dirname "$0")/.."
theme=testdata/tokyo-night.toml
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

go build -o ./mdv .

shot() { # name width height doc
	./mdv -frame -w "$2" -h "$3" -theme "$theme" "$4" | python3 tools/frame2svg.py >"docs/$1.svg"
	echo "docs/$1.svg  <- $4"
}

shot screenshot 86 31 testdata/screenshot.md
shot flowchart  86 46 testdata/flowchart.md
shot sequence   96 50 testdata/diagrams.md
shot models     96 33 testdata/models.md
shot charts     84 24 testdata/charts.md
shot people     84 26 testdata/people.md
./mdv -frame -images halfblock -w 76 -h 24 testdata/imgdemo.md | python3 tools/frame2svg.py >docs/images.svg
echo "docs/images.svg  <- testdata/imgdemo.md"

# A diagram wider than the pane: the viewer scrolls it instead of showing source.
./mdv -frame -hscroll 30 -w 76 -h 18 -theme "$theme" testdata/wide.md \
	| python3 tools/frame2svg.py >docs/wide.svg
echo "docs/wide.svg  <- testdata/wide.md (scrolled sideways)"

# The viewer screenshot also needs the table of contents open and a live search.
./mdv -frame -toc -find heading -w 84 -h 22 -theme "$theme" testdata/viewer.md \
	| python3 tools/frame2svg.py >docs/viewer.svg
echo "docs/viewer.svg  <- testdata/viewer.md (toc + search)"

rm -f ./mdv
