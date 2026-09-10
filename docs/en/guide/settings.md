---
description: Configure res-downloader resource and download settings, including the save directory, filenames, FFmpeg, certificates, proxies, and TLS interception policies.
---

# Settings

Settings are saved automatically. Restart the app after changing the listen address or port.

## Basic settings

- **Save Directory**: the default destination for new tasks. Changing it does not move existing files.
- **Filename Template**: controls download filenames and subdirectories. See [Filename template](#filename-template) for variables and examples.
- **Name Conflicts**: choose automatic numbering, overwrite, or skip. Automatic numbering is recommended.
- **Auto Intercept**: starts capturing automatically when the app launches.
- **Insert tail**: controls whether newly captured resources appear at the top or bottom of the list.
- **Clear cache and restart**: use this to recover from app problems. It clears certificates, settings, captured resources, task history, capture cache, and download temporary files in the current save directory. It keeps downloaded files, installed plugins, and temporary files in other save directories.
- **Open log folder**: opens the directory containing `app.log` on your system for diagnosing startup, capture, and download issues. Remove private information before sharing logs.

## Filename template

During a download, res-downloader replaces template variables with the resource information.

A template can contain literal text, <code v-pre>{{variables}}</code>, and `/` to create subdirectories within the save directory. Variable names are case-sensitive; use the lowercase names listed below.

### Default template

Leaving the field empty uses this default:

```text
{{title|default:resource|sanitize|truncate:80}}_{{date:20060102_150405}}.{{ext}}
```

It uses the resource title, substitutes the literal text `resource` if the title is empty, sanitizes incompatible filename characters, and limits the title to 80 characters. It then adds the download date, time, and output extension. Example: `Beautiful Chongqing_20260910_140509.mp4`.

### Available variables

Wrap a variable in double braces, for example <code v-pre>{{title}}</code>.

| Variable | Meaning | Example value |
| --- | --- | --- |
| `title` | Resource title | `Beautiful Chongqing` |
| `id` | Resource ID in the app | Depends on the resource |
| `kind` | Specific resource kind identifier | `media.video` |
| `plugin` | ID of the plugin that identified the resource | `com.example.video` |
| `host` | Main domain of the download track URL, usually without subdomains | `cdn.example.com` becomes `example.com` |
| `author` | Author information from resource metadata | `putyy` |
| `track` | ID of the track used for naming | `video-1080p` |
| `quality` | Track quality label | `1080p` |
| `width` / `height` | Track width / height in pixels | `1920` / `1080` |
| `bitrate` | Bitrate value supplied by the track, without a unit suffix | Depends on the track |
| `date` | Local date when the save path is generated; default format `20060102` | `20260910` |
| `time` | Local time when the save path is generated; default format `150405` | `140509` |
| `ext` | Download output extension, without the leading dot | `mp4` |
| `meta.*` | Resource metadata supplied by the plugin, such as `meta.site.author` | Depends on the plugin |

Author, quality, dimensions, and similar information depend on the resource and plugin and may be unavailable. Track variables prefer the track directly referenced by the download output. When the output comes from merging or other processing, they use the resource's primary track. Dates and times describe when the save path is generated, not when the content was published.

Missing fields, unknown variables, and width, height, or bitrate without a positive value produce empty strings. Use `default` to supply replacement text. `meta.*` supports strings, numbers, and booleans; objects and arrays do not become filenames directly. Metadata lookup checks the complete key first, then tries nested fields separated by dots. Available keys depend on the plugin.

### Filters and date formats

Append filters to a variable with `|`. Multiple filters run from left to right.

| Filter | Effect |
| --- | --- |
| `default:Unknown author` | Replaces an empty or whitespace-only value with `Unknown author` |
| `sanitize` | Replaces incompatible characters with `_` and handles surrounding whitespace, trailing dots, and system-reserved names |
| `truncate:80` | Keeps at most 80 Unicode characters; the argument must be an integer from 1 to 1000 |
| `lower` | Converts to lowercase |
| `upper` | Converts to uppercase |

For example, <code v-pre>{{author|default:Unknown author|sanitize}}</code> supplies a missing author before sanitizing the result. Add `sanitize` to titles and authors so that `/` or `\` within their values becomes `_` instead of creating unintended subdirectories. To supply replacement text when sanitizing leaves an empty value, put `default` after `sanitize`.

Add a colon after `date` or `time` to specify a format. Formats use Go's reference time digits: year `2006`, month `01`, day `02`, 24-hour clock hour `15`, minute `04`, and second `05`. Do not use `YYYY-MM-DD`.

| Expression | Example result |
| --- | --- |
| <code v-pre>{{date:2006-01-02}}</code> | `2026-09-10` |
| <code v-pre>{{date:2006/01}}</code> | `2026/09`, creating year and month directories |
| <code v-pre>{{time:15-04-05}}</code> | `14-05-09` |

Use `-` to separate time components in filenames; `:` is replaced with `_` during filename sanitization.

### Common templates

These examples assume a title of **Beautiful Chongqing**, author **putyy**, quality `1080p`, output extension `mp4`, and a local path-generation time of September 10, 2026 at 14:05:09. Results are relative to the save directory and assume no name conflicts.

**Title only**: produces `Beautiful Chongqing.mp4`.

```text
{{title|default:resource|sanitize|truncate:80}}.{{ext}}
```

**Group by author**: produces `putyy/Beautiful Chongqing.mp4`. A missing author uses the `Unknown author` directory.

```text
{{author|default:Unknown author|sanitize}}/{{title|default:resource|sanitize|truncate:80}}.{{ext}}
```

**Archive by year and month, retaining the download timestamp**: produces `2026/09/Beautiful Chongqing_20260910_140509.mp4`.

```text
{{date:2006/01}}/{{title|default:resource|sanitize|truncate:80}}_{{date:20060102_150405}}.{{ext}}
```

**Include quality**: produces `Beautiful Chongqing_1080p.mp4`. Missing quality uses `Original quality`.

```text
{{title|default:resource|sanitize|truncate:80}}_{{quality|default:Original quality|sanitize}}.{{ext}}
```

### Paths, extensions, and name conflicts

- **Paths**: use a path relative to the save directory. Do not enter drive letters, absolute paths, or parent directory segments (`..`). Both `/` and `\` are treated as directory separators; use `/` consistently.
- **Extensions**: end the template with <code v-pre>.{{ext}}</code>. When the output extension is known, the app appends it if it is missing or does not match. For example, MP4 output with a template ending in `.txt` results in `.txt.mp4`. Changing the template does not convert the file format.
- **Length and characters**: templates are limited to 4096 bytes. Each generated directory name and filename is sanitized and limited to 240 UTF-8 bytes, preserving the extension where possible. Long titles containing many Chinese characters or emoji may therefore be shortened further even with `truncate:80`.
- **Name conflicts**: controlled by **Name Conflicts** in Basic Setting. **Add a number** produces names such as `Beautiful Chongqing(1).mp4` and `Beautiful Chongqing(2).mp4`, also avoiding paths reserved by active downloads. **Overwrite existing file** allows replacing a file with the same name. **Skip download** stops that download when its target already exists or is reserved by another task.

Unclosed braces, empty variable expressions, unknown filters, and invalid truncation limits produce errors. If the operating system still rejects the generated filename, the app may use a fallback name. See [Downloaded filenames start with resource-](troubleshooting.md#downloaded-filenames-start-with-resource).

## Appearance

Switch between light, dark, and other built-in themes. Changes take effect immediately.

## Interface language

On first launch with no existing configuration, the app reads the system's preferred language: Chinese locales use Chinese; other locales, or a failed detection, use English. The choice is saved with the configuration and retained across launches and upgrades. Later changes to the system language or time zone do not switch it automatically.

Use the language button in the left menu to switch between Chinese and English. The change takes effect immediately and is saved automatically.

## Generic resource rules

These rules identify common images, videos, audio, and documents. The defaults usually work well; dedicated plugins take priority for the sites they support.

## Media processing

Merging separate audio and video tracks, remuxing, format conversion, audio extraction, and live recording require FFmpeg. Ordinary file downloads and the built-in HLS segment downloader work without it. FFmpeg is still required when a plugin uses it to download HLS, or when the downloaded media needs merging or remuxing.

FFmpeg is not bundled with the app; install it yourself. The two tools serve different purposes:

- **FFmpeg**: downloads, merges, remuxes, and processes media.
- **ffprobe**: reads media formats, tracks, and durations. It is usually distributed with FFmpeg.

#### Automatic detection

When the FFmpeg and ffprobe paths are empty, the app looks for `ffmpeg` and `ffprobe` in its process's `PATH` environment variable. The app inherits its environment when it starts, so restart it after installing FFmpeg or changing `PATH`, then click **Detect FFmpeg**.

Both FFmpeg and ffprobe must show **Available** to confirm that both tools are configured correctly.

#### Select executables manually

If you prefer not to configure `PATH`, or automatic detection fails, click **Choose file** beside each field and select the actual FFmpeg and ffprobe executables.

On Windows:

1. Choose a Windows distribution from the [official FFmpeg download page](https://ffmpeg.org/download.html) and extract it.
2. For FFmpeg, select `bin\ffmpeg.exe` inside the extracted directory.
3. For ffprobe, select `bin\ffprobe.exe` in the same directory.
4. Click **Detect FFmpeg** and confirm that both show **Available**.

On macOS and Linux, select the actual `ffmpeg` and `ffprobe` executables in your installation.

## Certificate

HTTPS capture requires the current certificate. Install, uninstall, or refresh its status under **Setting → Certificate**.

- Approve the UAC prompt on Windows; enter your system password when prompted on macOS or Linux.
- Phones need the certificate downloaded and installed separately.
- The app performs a one-time migration check for the shared certificate used by legacy versions. Follow any authorization prompt to remove it. Existing migration results are retained; routine upgrades preserve the current certificate and settings.

## Advanced settings

- **Listen address and port**: local access only by default. To connect a phone, use `0.0.0.0` and make sure the firewall allows the connection.
- **Upstream Proxy**: configure this when using another proxy tool alongside the app.
- **Download Proxy**: routes downloads through the upstream proxy when enabled.
- **Connections and Download Number**: try reducing these if downloads are unstable.
- **User-Agent and request headers**: normally keep the defaults. Do not share account information such as cookies.

## TLS interception policies

Choose which domains to capture and which to pass through. If a site becomes inaccessible or you do not want it captured, set its domain to **Pass through**.

For help with problems, see [Troubleshooting](troubleshooting.md).
