---
description: Configure res-downloader resource and download settings, including the save directory, filenames, FFmpeg, certificates, proxies, and TLS interception policies.
---

# Settings

Settings are saved automatically. Restart the app after changing the listen address or port.

## Basic settings

- **Save Directory**: the default destination for new tasks. Changing it does not move existing files.
- **Filename Template**: controls generated filenames. Keep the default if you are unfamiliar with templates.
- **Name Conflicts**: choose automatic numbering, overwrite, or skip. Automatic numbering is recommended.
- **Auto Intercept**: starts capturing automatically when the app launches.
- **Insert tail**: controls whether newly captured resources appear at the top or bottom of the list.
- **Clear cache and restart**: use this to recover from app problems. It clears certificates, settings, captured resources, task history, capture cache, and download temporary files in the current save directory. It keeps downloaded files, installed plugins, and temporary files in other save directories.
- **Open log folder**: opens the directory containing `app.log` on your system for diagnosing startup, capture, and download issues. Remove private information before sharing logs.

## Appearance

Switch between light, dark, and other built-in themes. Changes take effect immediately.

## Interface language

On first launch with no existing configuration, the app reads the system's preferred language: Chinese locales use Chinese; other locales, or a failed detection, use English. The choice is saved with the configuration and retained across launches and upgrades. Later changes to the system language or time zone do not switch it automatically.

Use the language button in the left menu to switch between Chinese and English. The change takes effect immediately and is saved automatically.

## Generic resource rules

These rules identify common images, videos, audio, and documents. The defaults usually work well; dedicated plugins take priority for the sites they support.

## Media processing

Merging audio and video, format conversion, HLS downloads, and live recording require FFmpeg. FFmpeg is not bundled with the app; install it yourself. The two tools serve different purposes:

- **FFmpeg**: downloads, merges, remuxes, and processes media.
- **ffprobe**: reads media formats, tracks, and durations. It is usually distributed with FFmpeg.

#### Automatic detection

When the FFmpeg and ffprobe paths are empty, the app looks for `ffmpeg` and `ffprobe` in its process's `PATH` environment variable. The app inherits its environment when it starts, so restart it after installing FFmpeg or changing `PATH`, then click **Detect FFmpeg**.

Both FFmpeg and ffprobe must show **Available** to confirm that both tools are configured correctly.

#### Select executables manually

If you prefer not to configure `PATH`, or automatic detection fails, click **Choose file** beside each field and select the actual FFmpeg and ffprobe executables. Do not select the installation directory or the `bin` directory itself.

On Windows:

1. Choose a Windows distribution from the [official FFmpeg download page](https://ffmpeg.org/download.html) and extract it.
2. For FFmpeg, select `bin\ffmpeg.exe` inside the extracted directory.
3. For ffprobe, select `bin\ffprobe.exe` in the same directory.
4. Click **Detect FFmpeg** and confirm that both show **Available**.

On macOS and Linux, select the actual `ffmpeg` and `ffprobe` executables in your installation. These files usually have no extension.

## Certificate

HTTPS capture requires the current certificate. Install, uninstall, or refresh its status under **Setting → Certificate**.

- Approve the UAC prompt on Windows; enter your system password when prompted on macOS or Linux.
- Phones need the certificate downloaded and installed separately.
- Upgrades automatically clean up old certificates and caches. Follow any authorization prompts.

## Advanced settings

- **Listen address and port**: local access only by default. To connect a phone, use `0.0.0.0` and make sure the firewall allows the connection.
- **Upstream Proxy**: configure this when using another proxy tool alongside the app.
- **Download Proxy**: routes downloads through the upstream proxy when enabled.
- **Connections and Download Number**: try reducing these if downloads are unstable.
- **User-Agent and request headers**: normally keep the defaults. Do not share account information such as cookies.

## TLS interception policies

Choose which domains to capture and which to pass through. If a site becomes inaccessible or you do not want it captured, set its domain to **Pass through**.

For help with problems, see [Troubleshooting](troubleshooting.md).
