#!/bin/sh
# Build the release archives and, optionally, tag and publish them.
#
#   tools/release.sh v0.1.0              build + tag locally
#   tools/release.sh v0.1.0 --publish    build + tag + push + gh release
#   tools/release.sh v0.1.0 --skip-tag   build only (CI, where the tag exists)
#
# Six static binaries, packaged with the README and the manual, plus checksums.
set -eu

cd "$(dirname "$0")/.."

version=${1:-}
case "$version" in
v[0-9]*.[0-9]*.[0-9]*) ;;
*)
	echo "usage: tools/release.sh vMAJOR.MINOR.PATCH [--publish|--skip-tag]" >&2
	exit 2
	;;
esac
shift || true

publish=no
skip_tag=no
for arg in "$@"; do
	case "$arg" in
	--publish) publish=yes ;;
	--skip-tag) skip_tag=yes ;;
	*) echo "unknown option: $arg" >&2; exit 2 ;;
	esac
done

if [ "$skip_tag" = no ]; then
	if [ -n "$(git status --porcelain)" ]; then
		echo "working tree is dirty; commit or stash first" >&2
		exit 1
	fi
	branch=$(git rev-parse --abbrev-ref HEAD)
	if [ "$branch" != main ]; then
		echo "releases are cut from main, not $branch" >&2
		exit 1
	fi
	if git rev-parse -q --verify "refs/tags/$version" >/dev/null; then
		echo "tag $version already exists; use --skip-tag to rebuild assets" >&2
		exit 1
	fi
fi

if ! command -v zip >/dev/null 2>&1 && ! command -v python3 >/dev/null 2>&1; then
	echo "need zip or python3 to package the Windows builds" >&2
	exit 1
fi

# zipdir packs a directory, using whichever zipper this machine has.
zipdir() {
	if command -v zip >/dev/null 2>&1; then
		(cd dist && zip -qr "$1" "$2")
	else
		(cd dist && python3 -m zipfile -c "$1" "$2")
	fi
}

echo "==> tests"
go test ./...

echo "==> building $version"
rm -rf dist
mkdir -p dist
ldflags="-s -w -X main.version=$version"
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
	os=${target%/*}
	arch=${target#*/}
	name="mdv_${version}_${os}_${arch}"
	bin=mdv
	[ "$os" = windows ] && bin=mdv.exe

	echo "    $os/$arch"
	CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags "$ldflags" -o "dist/$name/$bin" .

	cp README.md "dist/$name/"
	cp docs/mdv.1 "dist/$name/mdv.1" 2>/dev/null || true

	if [ "$os" = windows ]; then
		zipdir "$name.zip" "$name"
	else
		(cd dist && tar -czf "$name.tar.gz" "$name")
	fi
	rm -rf "dist/$name"
done

echo "==> checksums"
(cd dist && sha256sum ./*.tar.gz ./*.zip >checksums.txt)

if [ "$skip_tag" = yes ]; then
	echo "assets in dist/ (tag not touched)"
	exit 0
fi

echo "==> tagging $version"
git tag -a "$version" -m "mdv $version"

if [ "$publish" = no ]; then
	cat <<EOF
built and tagged $version.

to publish:
  git push origin main --follow-tags
  gh release create $version dist/*.tar.gz dist/*.zip dist/checksums.txt --title "mdv $version" --notes-from-tag
EOF
	exit 0
fi

echo "==> publishing"
git push origin main --follow-tags
gh release create "$version" dist/*.tar.gz dist/*.zip dist/checksums.txt \
	--title "mdv $version" --notes-from-tag
gh release view "$version" --json assets -q '.assets[].name'
