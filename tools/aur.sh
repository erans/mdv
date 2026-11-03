#!/bin/sh
# Publish the packaging in aur/ to the AUR.
#
#   tools/aur.sh v0.1.0    stamp that version and its tarball checksum, then push
#   tools/aur.sh           push whatever aur/PKGBUILD already says
#
# Called by .github/workflows/release.yml on every v* tag, so a release reaches
# the AUR without you doing anything. Run by hand after editing the PKGBUILD.
#
# This only rewrites pkgver/pkgrel/source/sha256sums. After changing any other
# field, regenerate the metadata first: cd aur && makepkg --printsrcinfo >.SRCINFO
set -eu
cd "$(dirname "$0")/.."

version=${1-}
version=${version#v}

if [ -n "$version" ]; then
	sum=$(curl -fsSL "https://github.com/erans/mdv/archive/refs/tags/v$version/mdv-$version.tar.gz" |
		sha256sum | cut -d' ' -f1)
	sed -i "s/^pkgver=.*/pkgver=$version/
		s/^pkgrel=.*/pkgrel=1/
		s/^sha256sums=.*/sha256sums=('$sum')/" aur/PKGBUILD
	sed -i "s/^\tpkgver = .*/\tpkgver = $version/
		s/^\tpkgrel = .*/\tpkgrel = 1/
		s|^\tsource = .*|\tsource = mdv-$version.tar.gz::https://github.com/erans/mdv/archive/refs/tags/v$version/mdv-$version.tar.gz|
		s/^\tsha256sums = .*/\tsha256sums = $sum/" aur/.SRCINFO
fi

url=${AUR_URL:-ssh://aur@aur.archlinux.org/mdv.git}
dir=$(mktemp -d)/mdv-aur
git clone "$url" "$dir" || {
	# the AUR makes the package on the first push
	echo "aur: nothing to clone, starting from an empty repository" >&2
	git init -q "$dir" && git -C "$dir" remote add origin "$url"
}
cp aur/PKGBUILD aur/.SRCINFO "$dir/"
cd "$dir"
git add PKGBUILD .SRCINFO
if git diff --cached --quiet; then
	echo "aur: nothing to publish"
	exit 0
fi
git -c user.name=mdv -c user.email=erans@users.noreply.github.com \
	commit -m "mdv $(sed -n 's/^pkgver=//p' PKGBUILD)-$(sed -n 's/^pkgrel=//p' PKGBUILD)"
git push origin HEAD:master
