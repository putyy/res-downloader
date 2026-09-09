---
description: Contribute to res-downloader through issue reports, documentation, plugins, and code. Learn how to preview, build, deploy, and maintain the bilingual VitePress docs.
---

# Contributing

You can contribute to res-downloader by reporting issues, improving documentation, developing plugins, or submitting code.

## Report issues

When opening a [GitHub issue](https://github.com/putyy/res-downloader/issues), include as much of the following as possible:

- Operating system and app version.
- Relevant plugins and their versions.
- Reproducible steps.
- Actual and expected results.
- Sanitized error messages or logs.

Do not publish cookies, Authorization headers, account details, administrator passwords, privately signed download URLs, or other sensitive data.

## Improve documentation

User documentation should explain where to find a control, what an option does, and how to solve a problem. Avoid internal implementation details that do not affect usage. Corrections, screenshots, improved installation steps, and translations can be submitted directly as pull requests.

### Preview documentation locally

The documentation uses VitePress with dependencies managed independently in `docs/`, separate from the desktop app's `frontend/`. Install Node.js 22 or later, then run from the repository root:

```bash
cd docs
npm ci
npm run dev
```

The default address is `127.0.0.1:8088`. Open <http://127.0.0.1:8088> in a browser to preview the site. Markdown edits update automatically; press `Ctrl+C` to stop the server. Once dependencies are installed, site styles, scripts, and search do not depend on external CDNs.

To change the port or allow LAN access, run from `docs/`:

```bash
npm run dev -- --port 9000
npm run dev -- --host 0.0.0.0
```

### Build and deploy documentation

Run static checks and the build from `docs/`:

```bash
npm run check
npm run build
```

Output is written to `docs/.vitepress/dist/`. Each document becomes a separate HTML file containing its full text; the build also generates a sitemap and local search indexes. Document URLs end in `.html`, and directory homepages use `/`, for example `/zh/guide/installation.html` and `/en/development/plugin-sdk.html`.

Deploy the contents of `.vitepress/dist/` to the static server's website root. Do not continue publishing the `docs/` source directory directly. The server must support `index.html` as the default directory file and return HTTP 404 for missing paths. You can use the generated `404.html` as the error page; do not fall back to the homepage for every path. Node.js is not needed in production.

The configured site URL is `https://res.putyy.com/`. If the domain changes, update `hostname` in `.vitepress/config.mts` and the sitemap address in `public/robots.txt`. This configuration deploys at the domain root and does not redirect old Docsify hash links.

To inspect the build manually, run from `docs/`:

```bash
npm run preview
```

Before deployment, manually check desktop and mobile layouts, light and dark themes, Chinese and English search, language switching, heading links, code copying, and Plugin SDK downloads. After deployment, check direct subpage access and refresh, HTTP 404 responses, and submit `https://res.putyy.com/sitemap.xml` to search engines.

### Documentation structure and assets

- `docs/` is the documentation project root. `zh/` and `en/` contain the Chinese and English pages. Each has an `index.md` homepage, a `guide/` for user guides, and a `development/` for architecture, contribution, and plugin documentation. The root `index.md` redirects visitors to `/zh/`. Project setup instructions live in `docs/README.md`, which is excluded from site pages.
- `.vitepress/config.mts` manages the build, shared assets, and locale registration. `.vitepress/locales/zh.mts` and `en.mts` contain the respective navigation, outline, search, and interface text. Theme styles live in `.vitepress/theme/custom.css`.
- Maintain each page's `description` in frontmatter. The title defaults to the first level-one heading and can be overridden with `title`. The build uses these values for page titles, descriptions, canonical URLs, and sharing metadata.
- Use relative `.md` links between pages, with `../guide/` or `../development/` across categories. When moving pages, update navigation, homepage links, both repository READMEs, examples, and project collaboration rules.
- Images, icons, `robots.txt`, `.nojekyll`, and SDK files live in `public/`. Vite publishes them unchanged without an extra copy script. Pages use root paths such as `/images/show.png` and `/plugin-sdk/plugin-v1.schema.json`, shared by all languages. Repository READMEs use relative paths under `docs/public/`.
- Do not commit `node_modules/`, `.vitepress/cache/`, or `.vitepress/dist/`. Commit `package-lock.json`, `public/`, and all page sources.

### Multilingual documentation

Chinese and English are registered as `locales.zh` and `locales.en`. Pages live in `docs/zh/` and `docs/en/` and use `/zh/` and `/en/` URLs. The site root redirects to the Chinese homepage through static HTML. This entry page is excluded from search and the sitemap, and its canonical URL points to `/zh/`. The static server can also be configured to issue an HTTP redirect from `/` to `/zh/`.

To add another language:

1. Add a homepage and translated pages under `docs/<language-code>/`, preserving the Chinese version's relative directories and filenames. For example, `en/guide/getting-started.md` corresponds to `zh/guide/getting-started.md`.
2. Create a locale configuration based on `.vitepress/locales/zh.mts` and `en.mts`. Maintain `label`, `lang`, the site description, navigation, sidebar, footer, and interface text. Navigation and homepage buttons must use language-prefixed URLs such as `/en/guide/getting-started`.
3. Import and register it in `config.mts` under `locales`. Register search translations under the same key in `themeConfig.search.options.locales`. Search indexes are generated per language; tokenization is shared so indexing and browser search use the same rules.
4. Translate each page's `title`, `description`, body, and link text, and update heading links. Use relative links within a language. Continue using root paths for shared images and SDK files rather than duplicating them in language directories.

The language menu opens the corresponding translation of the current page. Register only languages with complete matching pages and navigation targets to avoid 404s after switching. Content is not translated automatically, and missing translations are not filled with Chinese. Keep both language versions synchronized when adding or changing documentation.

`.vitepress/seo.mts` generates canonical URLs, sharing titles, and language metadata for each page's locale. It emits `hreflang` only when multiple translations actually exist, with Chinese as the default language. The sitemap also associates existing translations. After adding a language, run a static build and manually check switching, navigation, search, direct links, and page rendering.

## Develop plugins

For new site support, develop a standalone plugin instead of adding site checks to the generic downloader. Start with the [plugin developer guide](plugins.md) and [examples](https://github.com/putyy/res-downloader/tree/master/examples/plugins), and include at least one offline fixture without private data.

Public plugins can follow the [extension store publishing guide](extension-store.md) without merging their code into the main project.

## Contribute code

Bug fixes and small documentation updates can be submitted directly as pull requests. For new features, architecture changes, or large refactors, open an issue first to align the approach with the project.

The root [CONTRIBUTING.md](https://github.com/putyy/res-downloader/blob/master/CONTRIBUTING.md) (Chinese) is authoritative for PR scope, title format, and pre-submission checks.

### Desktop frontend development

Frontend dependencies are maintained independently in `frontend/` and require **Node.js 22.12 or later**. Windows and macOS release workflows and the Linux build image use Node.js 22. Run from the repository root:

```bash
cd frontend
npm ci
npm run check
npm run build
```

`check` runs `vue-tsc -b` for Vue pages and the Vite configuration. `build` runs the same checks before producing `frontend/dist/`. Wails uses `npm ci` to install from the lockfile. Commit `package.json` and `package-lock.json` together when updating dependencies.

`tsconfig.json` only organizes project references. Page and browser types belong in `tsconfig.app.json`; build configuration and Node types belong in `tsconfig.node.json`. Vite plugins generate `auto-imports.d.ts` and `components.d.ts`, which are committed so a fresh checkout can run type checks immediately. Include any declaration changes produced by the build. Configure your IDE to use TypeScript 5.9 from `frontend/node_modules/typescript`; the documentation project keeps its own dependencies.

The Vite configuration explicitly preserves the previous JavaScript compilation targets and continues using esbuild for CSS minification. Tailwind stays on 3.4. These choices reduce the effect of build-tool changes on existing WebViews; they do not establish compatibility with every older system.

Manually verify interactions on supported Windows, macOS, and Linux systems: app startup, the main and settings pages, language and theme switching, certificate authorization, plugin management, resource lists, download tasks, and image, audio/video, HLS / FLV previews. During development, also check page updates and Go binding calls through `wails dev`.

### Windows installer scripts

Installation directory validation and uninstall protections live in `build/windows/installer/safety.nsh`, included by `project.nsi`. Do not implement these protections in `wails_tools.nsh`, which Wails regenerates during builds.

When compiling a Fixed WebView2 installer, NSIS automatically runs `go run ./runtimefiles`, using the existing Go toolchain to embed an exact runtime file list in the uninstaller. Regular installers do not invoke the generator, and it does not run during user installation or uninstallation. The generator lives in `build/windows/installer/runtimefiles/`, uses only the Go standard library, and accepts `-DRESD_GO=<Go-executable-path>` to select the build tool.

Before uninstalling, the registry installation path is checked against the installation marker, and directory links are rejected. Deletion is limited to the installer's embedded file list. Nonempty directories, user data, and unknown runtime leftovers are retained. Regular upgrades do not delete the entire old Fixed WebView2 directory. Old uninstallers do not gain these protections automatically; install over them with a new package to update them.

After changing uninstall logic, statically compile the regular amd64, arm64, and Fixed WebView2 branches. Then manually verify in a Windows VM with a restorable snapshot: normal and silent uninstalls preserve extra files; moving the uninstaller or removing the installation marker or registry entry prevents all deletion; system directories, junctions, and symlinks are rejected; files in use leave the records needed to retry. Use temporary test directories and disposable placeholder files, never real system or user-data directories, to reproduce deletion issues.
