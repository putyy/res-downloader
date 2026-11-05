#!/usr/bin/env bash
# Example for Ubuntu with AppArmor ABI 4. Run with sudo; never run the app as root.
# Usage: sudo bash appimage-apparmor.sh install /absolute/path/res-downloader.AppImage
#        sudo bash appimage-apparmor.sh remove
set -euo pipefail

profile=/etc/apparmor.d/res-downloader-appimage
marker='# Managed by res-downloader appimage-apparmor.sh'
[[ $EUID == 0 ]] || { echo 'Run this configuration script with sudo.' >&2; exit 1; }
command -v apparmor_parser >/dev/null || { echo 'Install the apparmor package first.' >&2; exit 1; }
if [[ -e "$profile" || -L "$profile" ]]; then
  [[ ! -L "$profile" ]] && grep -qxF "$marker" "$profile" || {
    echo "Refusing to replace an unmanaged profile: $profile" >&2; exit 1;
  }
fi

case "${1:-}" in
  install)
    [[ $# == 2 && $2 == /* ]] || { echo 'Provide the absolute AppImage path.' >&2; exit 1; }
    [[ -f /etc/apparmor.d/abi/4.0 ]] || { echo 'AppArmor ABI 4 is required.' >&2; exit 1; }
    appimage=$(realpath -e -- "$2")
    [[ -f "$appimage" && -x "$appimage" ]] || { echo 'AppImage must be an executable file.' >&2; exit 1; }
    # Deliberately restrict this example to literal paths, with no policy syntax.
    [[ "$appimage" =~ ^/[-a-zA-Z0-9_./\ ]+\.AppImage$ ]] || {
      echo 'Use a path containing only ASCII letters, digits, spaces, /, ., _, and -.' >&2; exit 1;
    }
    temporary=$(mktemp)
    trap 'rm -f "$temporary"' EXIT
    cat > "$temporary" <<EOF
$marker
abi <abi/4.0>,
include <tunables/global>
profile res-downloader-appimage "$appimage" flags=(unconfined) {
    userns,
}
EOF
    apparmor_parser --skip-kernel-load --skip-read-cache "$temporary"
    apparmor_parser --replace --skip-read-cache "$temporary"
    install -m 644 "$temporary" "$profile"
    echo 'Profile installed. Start the AppImage as your normal desktop user.'
    ;;
  remove)
    [[ $# == 1 ]] || { echo 'Usage: appimage-apparmor.sh remove' >&2; exit 1; }
    if [[ -f "$profile" ]]; then
      if [[ -r /sys/kernel/security/apparmor/profiles ]] &&
         grep -q '^res-downloader-appimage (' /sys/kernel/security/apparmor/profiles; then
        apparmor_parser --remove "$profile"
      fi
      rm -f "$profile"
    fi
    ;;
  *) echo 'Usage: appimage-apparmor.sh install /absolute/path/res-downloader.AppImage | remove' >&2; exit 1 ;;
esac
