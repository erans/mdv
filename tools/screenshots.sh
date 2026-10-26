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
shot sequence   96 36 testdata/diagrams.md
shot models     96 33 testdata/models.md
./mdv -frame -images halfblock -w 76 -h 24 testdata/imgdemo.md | python3 tools/frame2svg.py >docs/images.svg
echo "docs/images.svg  <- testdata/imgdemo.md"

rm -f ./mdv
