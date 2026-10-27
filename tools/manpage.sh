#!/bin/sh
# Write the mdv(1) manual from the binary that ships it, so the page can never
# drift from the flags.
set -eu
cd "$(dirname "$0")/.."
go build -o ./mdv .
./mdv -man >docs/mdv.1
rm -f ./mdv
echo "docs/mdv.1 written ($(wc -l <docs/mdv.1) lines)"
