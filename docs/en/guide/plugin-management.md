---
description: Install, enable, configure, update, and uninstall res-downloader plugins, review their permissions, and troubleshoot plugin issues.
---

# Plugin Management

Plugins improve resource detection and downloading for specific sites. Some official plugins are included with the app, and community plugins can be installed separately.

## Install plugins

Open **Plugins** to install from the extension store or select a local ZIP file.

Descriptions on installed plugin and store cards show up to two lines. Hover over a description to read it in full.

Before installing, check that the developer, requested domains, and permissions fit the plugin's purpose. Community plugins are provided by third parties and are not necessarily security-reviewed by the project.

## Enable and configure

- A disabled plugin stops processing new resources.
- Some plugins provide settings such as quality or capture scope.
- After changing settings, save or reload as instructed on the page.
- Plugin debug logging is off by default. If a plugin offers **Enable logging**, turn it on for troubleshooting and off when finished.

## Update and uninstall

Plugin cards show a notice when an update is available. The extension store lists updatable plugins first.

If an update fails, retry or roll back to the previous version. Uninstalling a plugin does not delete downloaded files.

## Troubleshoot a plugin

Check the following in order:

1. Is the plugin enabled?
2. Does its card show an error?
3. Have you reopened the page so that it generates new requests?
4. Are the plugin settings correct?
5. Has the target site changed?

If the issue persists, contact the plugin developer with the app version, plugin version, reproduction steps, and error message. Remove private information such as cookies and account details first.

To create plugins, see the [plugin developer guide](../development/plugins.md).
