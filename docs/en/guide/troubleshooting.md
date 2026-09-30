---
description: Troubleshoot res-downloader installation, certificates, resource capture, phone connections, and downloads. Find logs and resolve CLI / MCP and live recording issues.
---

# Troubleshooting

## Too many WeChat Channels resources

Under **Plugins**, turn off the WeChat Channels plugin's full capture mode. Clear the resource list and reopen the target detail page.

## Large resource list warning

At 1,500 resource records, the app warns that a large list may increase memory usage and affect stability. It warns again for every additional 500 records. The count includes collection children regardless of pagination, filters, or expanded rows. Startup recovery and imports also check the count; a batch crossing multiple thresholds produces only one warning.

This does not mean the app has detected low memory, and it does not clear resources automatically. Remove unneeded resources using the list's existing cleanup controls. The next warning threshold is recalculated from the remaining count.

## A website or app cannot be captured

Some apps do not support proxy capture or require a dedicated plugin. First confirm that capture is enabled, the certificate is installed, and the target domain is not set to **Pass through**.

## Reddit does not load correctly with capture enabled

If pages on `reddit.com` fail to load with capture enabled, add and enable this policy under **Setting → Advanced Setting → TLS Interception Policies**:

| Setting | Value |
| --- | --- |
| Policy name | `*.reddit.com` |
| Action | Pass through |
| Domains | Add `*.reddit.com` and `*.redditstatic.com` separately |
| Domains excluded by this policy | Leave empty |

Add each domain as a separate entry, rather than combining them into a comma-separated value. Place this policy after the general **Decrypt and capture** policy. When several policies match the same domain, the later match takes precedence.

Reopen Reddit after changing the settings so that new connections use the policy. Pass-through preserves the encrypted HTTPS connections for these domains; the app no longer decrypts or captures their contents. See [TLS interception policies](settings.md#tls-interception-policies) for more information.

## Certificate still shown as missing after installation

Open **Setting → Certificate**, reinstall the certificate, and refresh its status. On Windows, approve the UAC prompt. On phones, manually remove the old certificate before installing the new one.

## Connect a phone

1. Connect the phone and computer to the same network.
2. Set the listen address to `0.0.0.0`, restart the app, and enable capture.
3. Set the phone's proxy to the computer's LAN IP address and the app's port.
4. Download the current certificate on the phone, install it, and enable trust.

Some phone apps may still be impossible to capture even after setup.

## No internet access after quitting the app

Disable the HTTP / HTTPS proxy in your system's network settings, then restart the app.

## App fails to start

Check the [Installation guide](installation.md) to confirm that the package matches your operating system and CPU architecture, and note any startup error. If you cannot open the main interface, use the [log locations](#find-application-logs) to find `app.log` directly. Provide sanitized logs along with your system and app versions. If no log file exists, include an error screenshot and the installer filename.

## Settings behave incorrectly but the interface opens

Use **Clear cache and restart** under **Setting → Basic Setting**, approving authorization when prompted. This resets settings and certificates and clears captured resources, task history, and other data. Review the full scope under [Basic settings](settings.md#basic-settings) before proceeding. Downloaded files and installed plugins are preserved.

## Retry a failed update check

Once the connection is restored, select **Check for updates** under **Setting → Basic Setting** without restarting the app. Update checks also follow **Download Proxy**, so confirm that the configured upstream proxy works when enabled. See [Basic settings](settings.md#basic-settings) for the settings location.

If the update service is not providing valid version information, try again later or visit the website. Include the update source error from the application log when reporting the problem. The server may be missing valid metadata; resetting local settings will not fix that.

## Debian / Ubuntu installation permission notice {#debian-ubuntu-apt-permission}

When using `sudo apt install ./package.deb`, you may see a notice like this at the end:

```text
N: Download is performed unsandboxed as root as file '…deb' couldn't be accessed by user '_apt'. - pkgAcquire::Run (13: Permission denied)
```

APT prefers the low-privilege `_apt` user when acquiring packages, even when the command is run with `sudo`. If the package is unreadable, or a parent directory such as your home or Desktop directory prevents `_apt` from accessing it, APT falls back to reading the local file as root. This notice alone does not mean installation failed, and it does not describe how the application itself is sandboxed.

First, check the current installation status:

```bash
dpkg -s res-downloader
```

If the output includes `Status: install ok installed`, the package is currently installed and configured. Also check that `Version:` matches the expected version. There is no need to reinstall solely because of this notice. If other errors report unmet dependencies or another problem, troubleshoot using the full output.

To avoid the notice, copy the package to a directory accessible to `_apt` and grant read permission before installing. This example uses the amd64 package for `4.0.0`; replace the path, version, and architecture with those of your downloaded file:

```bash
cp ~/Desktop/res-downloader_4.0.0_linux_amd64.deb /tmp/
chmod 644 /tmp/res-downloader_4.0.0_linux_amd64.deb
sudo apt install /tmp/res-downloader_4.0.0_linux_amd64.deb
```

You do not need to relax permissions on your entire home directory. `sudo dpkg -i` bypasses APT's package acquisition process, so it does not produce this `_apt` notice, but it also does not download missing dependencies. See the [Debian / Ubuntu installation guide](installation.md#debian-ubuntu) for the recommended method.

## Retry a failed Windows installation

If the first installation fails because of disk space, locked files, or WebView2 permission setup, resolve the reported problem and run the installer again with the same destination. If the directory has a valid installation record or interrupted-installation record, installation can continue after validation. You do not need to delete the files left behind.

A nonempty directory without valid records is rejected when its ownership cannot be established. Keep that directory and install into another empty directory, or report the problem through [GitHub Issues](https://github.com/putyy/res-downloader/issues). Do not delete the entire directory to bypass the check.

## Cannot uninstall on Windows

If files are in use, close the app or restart the computer and try again. If the installation record is invalid, download the latest installer, try installing over the original directory, then uninstall. If the installer rejects that directory, keep it and report the error through [GitHub Issues](https://github.com/putyy/res-downloader/issues). Do not delete the entire directory directly.

Uninstall removes recorded application and fixed WebView2 files, preserving downloads, settings, plugins, and other unknown files. See [Installation](installation.md#windows) for switching between WebView2 installer variants.

## CLI or MCP cannot connect

CLI and MCP require the updated desktop app to stay running under the same operating system user. For missing connection files, expired sessions, agent sandbox permissions, or empty resource lists, see [CLI and MCP: connection and troubleshooting](automation.md#connection-and-troubleshooting).

## Find application logs

Release builds write runtime logs to `app.log`. If you can open the main interface, click **Open log folder** under **Setting → Basic Setting**. Reproduce the problem and quit the app before copying the log for troubleshooting.

### Windows

Log path:

```text
%APPDATA%\res-downloader\logs\app.log
```

Press `Win + R`, enter `%APPDATA%\res-downloader\logs`, and press Enter to open the log folder in File Explorer. You can also read the last 200 lines in PowerShell:

```powershell
Get-Content "$env:APPDATA\res-downloader\logs\app.log" -Tail 200
```

### macOS

Log path:

```text
~/Library/Preferences/res-downloader/logs/app.log
```

In Finder, press `Command + Shift + G`, enter `~/Library/Preferences/res-downloader/logs`, and press Enter. Alternatively, open the folder or read the last 200 lines in Terminal:

```bash
open ~/Library/Preferences/res-downloader/logs
tail -n 200 ~/Library/Preferences/res-downloader/logs/app.log
```

### Linux

Default log path:

```text
~/.config/res-downloader/logs/app.log
```

If `XDG_CONFIG_HOME` is set, logs are saved at `$XDG_CONFIG_HOME/res-downloader/logs/app.log`. Read the last 200 lines in a terminal:

```bash
tail -n 200 "${XDG_CONFIG_HOME:-$HOME/.config}/res-downloader/logs/app.log"
```

If the log folder or `app.log` does not exist, the app may have exited before logging was initialized. Include your OS version, system architecture, app version, installer filename, reproduction steps, and screenshots when reporting the issue.

Before sharing logs, inspect and redact account details, cookies, tokens, resource URLs, local usernames, file paths, and other private information.

## macOS says the app is damaged and cannot be opened

Confirm that the installer came from the project's official releases, then allow the app under **System Settings → Privacy & Security**. If it still cannot open, run:

```bash
sudo xattr -d com.apple.quarantine /Applications/res-downloader.app
```

## Slow downloads or failed large files

Check free disk space, whether the link has expired, and whether the download proxy works. Expired WeChat Channels links must be captured again.

## Download reports a filename template error

Check **Settings → Basic Settings → Filename Template**. The template must produce a path relative to the save directory. It cannot start with `/` or a drive letter or contain a parent directory segment (`..`). The settings page rejects errors it can detect in advance and shows the reason below the field. Resource titles and plugin metadata are only known during a download and can still produce an unsafe path. Use `sanitize` for titles and authors and add `default` where needed; see [Filename template](settings.md#filename-template).

If the template looks correct but the error persists, provide the full error, operating system, save directory, template, and resource title after removing private information.

## Downloaded filenames start with resource-

The app first generates a name from the filename template, removing incompatible characters and shortening long names. If the operating system still rejects it, the final file placement step uses `resource-<short-id>.<extension>` to preserve the downloaded and processed data. The actual saved path is shown in the download center. If both placement attempts fail, temporary output remains in `.res-downloader-work` inside the current save directory. Remove it by deleting the failed task, clearing finished tasks, or using **Clear cache and restart**. To investigate why the original name was rejected, provide sanitized logs.

## Live recording does not work

Install FFmpeg, then run detection under **Setting → Media Processing**.

## Still need help

Report the issue through [GitHub Issues](https://github.com/putyy/res-downloader/issues), including your operating system, app version, reproduction steps, and error message. Remove private information such as account details and cookies first.
