<p align="center">
  <strong>Wox</strong> — <em>A launcher that simply works.</em>
</p>

<div align="center">

[**简体中文版**](README.zh_CN.md)

</div>

---

# English

**Wox** is a free, open-source **cross-platform launcher** for Windows, macOS, and Linux — a fast, keyboard-driven alternative to Alfred, Launchy, Raycast, and PowerToys Run. Summon it with a hotkey, start typing, and instantly search your apps, files, folders, browser bookmarks, the web, and more. It ships with a rich plugin and theme ecosystem, so you can extend it with JavaScript, Python, C#, and other languages.

This repository is a **fork of [Wox](https://github.com/poplknight/Wox)** with a curated set of modifications — see [Changes in this fork](#changes-in-this-fork).

## Key features

- **Search everything** — applications, folders, files, browser bookmarks, web searches, and more, with fuzzy and pinyin matching for Chinese program names.
- **Plugins & themes** — install plugins and themes from the store, or build your own; plugin development in JavaScript, Python, C#, and more.
- **Fast & portable** — no installation required; launch from a single executable.
- **Extensible workflows** — system plugins cover clipboard, notes, screenshots, quick jump, window management, and more.

## Changes in this fork

Compared with the upstream [Wox](https://github.com/poplknight/Wox), this fork:

- Disables color, calculator, timer, emoji, notes, quick jump, preview, selection, update, dictation, reminder, feedback, unit converter, menu, theme, and cloud sync. The code remains; it is simply no longer registered.
- Keeps the plugin indicator, plugin installer, and automation test plugins loading, but hides them from the installed-plugin list.
- Turns off automatic updates and removes the update entry from Settings; the cloud-sync entry is removed as well.
- Adds sync below the existing backups and logs. Export stores all current settings in a single file; importing that file restores them.
- For a system command typed with only its trigger keyword, the results list the available sub-commands. Commands that still expect input complete the query on Enter, then wait for arguments.
- Plugin management lists install / update / uninstall / create plus local-plugin commands when only the trigger keyword is typed; Enter enters the corresponding command.
- Shell history appears only while nothing has been typed after `>`. As soon as you start typing, the results keep only the command to execute.
- In the Folder plugin, pressing Tab completes the currently selected directory into the query and continues listing the next level.
- Content starting with `http://` or `https://` is recognized directly as a URL, including Chinese domain names.
- Adds a new local-info plugin with trigger keyword `info`. It first lists host, IP, CPU, memory, network, energy, disk, and user; Enter enters the detail, and Enter on a detail item copies it. Systems without energy data do not show energy.
- When the media player cannot read the currently playing item, the results offer an install option. The Settings page shows the install status above the keywords; macOS and Windows can install from there.

## Building from source

Requirements: Go, Node.js + pnpm, uv, and platform tooling (see the `Makefile` `_check_deps` target).

```bash
make build                 # full build + package for the current platform
make -C wox.core build     # build the core binary into release/
```

## Documentation & community

- Upstream project: [Wox-launcher/Wox](https://github.com/poplknight/Wox)
- Official site: [woxlauncher.com](https://www.woxlauncher.com/)

## License

[GPL-3.0](LICENSE) — fork of [Wox](https://github.com/poplknight/Wox).
