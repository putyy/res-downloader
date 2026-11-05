#!/bin/sh
set -e

profile=/etc/apparmor.d/res-downloader
template=/usr/share/res-downloader/apparmor/res-downloader

# Do not put ABI 4 syntax in /etc/apparmor.d on older AppArmor systems.
# Generate this file only where supported; keep the template in package data.
case "${0##*/}:${1:-}" in
    *.postinst:configure|postinst:configure)
        if command -v apparmor_parser >/dev/null 2>&1 && [ -f /etc/apparmor.d/abi/4.0 ]; then
            apparmor_parser --skip-kernel-load --skip-read-cache "$template"
            install -m 644 "$template" "$profile"
            if [ -r /sys/kernel/security/apparmor/profiles ]; then
                # Fail configuration visibly if policy loading fails on an active host.
                apparmor_parser --replace --skip-read-cache --write-cache "$profile"
            fi
        elif [ -r /proc/sys/kernel/apparmor_restrict_unprivileged_userns ] &&
             [ "$(cat /proc/sys/kernel/apparmor_restrict_unprivileged_userns)" = 1 ]; then
            echo 'res-downloader: install AppArmor with ABI 4 support, then run sudo dpkg --configure res-downloader.' >&2
            exit 1
        fi
        ;;
    *.postrm:remove|postrm:remove|*.postrm:purge|postrm:purge)
        if [ -f "$profile" ]; then
            if command -v apparmor_parser >/dev/null 2>&1 &&
               [ -r /sys/kernel/security/apparmor/profiles ] &&
               grep -q '^res-downloader (' /sys/kernel/security/apparmor/profiles; then
                apparmor_parser --remove "$profile"
            fi
            rm -f "$profile"
        fi
        # In particular, postrm upgrade must not unload the new version's policy.
        ;;
esac

# Shared by postinst and postrm. Cache tools are optional on minimal desktops.
if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database /usr/share/applications || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
    gtk-update-icon-cache -f -q -t /usr/share/icons/hicolor || true
fi
