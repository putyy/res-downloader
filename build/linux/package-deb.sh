#!/usr/bin/env bash
set -euo pipefail
umask 022

# Run in the same Debian/Ubuntu environment that compiled the CGO executable.
for tool in dpkg-deb dpkg readelf desktop-file-validate; do
  command -v "$tool" >/dev/null || { echo "Missing packaging tool: $tool" >&2; exit 1; }
done
root=$(pwd)
binary="$root/bin/res-downloader"
arch=${ARCH:?ARCH must be amd64 or arm64}
version=${PACKAGE_VERSION:?PACKAGE_VERSION must be a Debian package version}
case "$arch" in
  amd64) machine='Advanced Micro Devices X86-64' ;;
  arm64) machine='AArch64' ;;
  *) echo "Unsupported architecture: $arch" >&2; exit 1 ;;
esac
export LC_ALL=C
dpkg --validate-version "$version"
[[ $(dpkg --print-architecture) == "$arch" ]] || { echo 'Package inside a container matching ARCH.' >&2; exit 1; }
readelf -h "$binary" | grep -F "$machine" >/dev/null
desktop-file-validate build/linux/application.desktop

work=$(mktemp -d "$root/bin/.deb-package.XXXXXX")
trap 'rm -rf "$work"' EXIT
stage="$work/package"
install -Dm755 "$binary" "$stage/usr/bin/res-downloader"
install -Dm644 build/linux/application.desktop "$stage/usr/share/applications/res-downloader.desktop"
install -Dm644 build/linux/Debian/usr/share/icons/hicolor/256x256/apps/res-downloader.png \
  "$stage/usr/share/icons/hicolor/256x256/apps/res-downloader.png"
install -Dm755 build/linux/deb-maintainer.sh "$stage/DEBIAN/postinst"
install -Dm755 build/linux/deb-maintainer.sh "$stage/DEBIAN/postrm"
install -Dm644 build/linux/res-downloader.apparmor \
  "$stage/usr/share/res-downloader/apparmor/res-downloader"

# Keep portable top-level runtime dependencies instead of importing the build
# distribution's direct library package names (for example GLib's t64 suffix).
size=$(du -sk "$stage/usr" | awk '{print $1}')
cat > "$stage/DEBIAN/control" <<EOF
Package: res-downloader
Version: $version
Section: net
Priority: optional
Architecture: $arch
Maintainer: putyy <putyy@qq.com>
Installed-Size: $size
Depends: libgtk-4-1, libwebkitgtk-6.0-4, gstreamer1.0-libav, gstreamer1.0-plugins-good, ca-certificates, libnss3-tools, libglib2.0-bin, gsettings-desktop-schemas
Recommends: sudo, pkexec, apparmor
Homepage: https://github.com/putyy/res-downloader
Description: Capture resources and manage downloads
EOF

dpkg-deb --root-owner-group --build "$stage" "$work/res-downloader.deb"
# Inspect the archive, never execute or install the application during packaging.
dpkg-deb --extract "$work/res-downloader.deb" "$work/check"
desktop-file-validate "$work/check/usr/share/applications/res-downloader.desktop"
test -x "$work/check/usr/bin/res-downloader"
test -s "$work/check/usr/share/icons/hicolor/256x256/apps/res-downloader.png"
cmp build/linux/res-downloader.apparmor "$work/check/usr/share/res-downloader/apparmor/res-downloader"
dpkg-deb --control "$work/res-downloader.deb" "$work/control-check"
cmp build/linux/deb-maintainer.sh "$work/control-check/postinst"
cmp build/linux/deb-maintainer.sh "$work/control-check/postrm"
sh -n "$work/control-check/postinst"
sh -n "$work/control-check/postrm"
test ! -e "$work/check/usr/local"
test "$(dpkg-deb --field "$work/res-downloader.deb" Architecture)" = "$arch"
test "$(dpkg-deb --field "$work/res-downloader.deb" Version)" = "$version"
if find "$work/check" -name .DS_Store -o -name .gitkeep | grep -q .; then
  echo 'Unexpected development files in DEB.' >&2
  exit 1
fi
mv -f "$work/res-downloader.deb" "$root/bin/res-downloader.deb"
echo "DEB ready: bin/res-downloader.deb ($arch, $version)"
