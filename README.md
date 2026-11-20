<p align="center">
  <strong>Wox</strong> — <em>A launcher that simply works.</em>
</p>

<div align="center">

[**English**](#english) · [**简体中文**](#简体中文)

</div>

---

# English

**Wox** is a free, open-source **cross-platform launcher** for Windows, macOS, and Linux — a fast, keyboard-driven alternative to Alfred, Launchy, Raycast, and PowerToys Run. Summon it with a hotkey, start typing, and instantly search your apps, files, folders, browser bookmarks, the web, and more. It ships with a rich plugin and theme ecosystem, so you can extend it with JavaScript, Python, C#, and other languages.

This repository is a **fork of [Wox](https://github.com/Wox-launcher/Wox)** with a curated set of modifications — see [Changes in this fork](#changes-in-this-fork).

## Key features

- **Search everything** — applications, folders, files, browser bookmarks, web searches, and more, with fuzzy and pinyin matching for Chinese program names.
- **Plugins & themes** — install plugins and themes from the store, or build your own; plugin development in JavaScript, Python, C#, and more.
- **Fast & portable** — no installation required; launch from a single executable.
- **Extensible workflows** — system plugins cover clipboard, notes, screenshots, quick jump, window management, and more.

## Changes in this fork

Compared with the upstream [Wox](https://github.com/Wox-launcher/Wox), this fork:

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

## Version & release

Current version: **v0.0.1**

A prebuilt **macOS (Apple Silicon, arm64)** package is available in [`release/`](release/):

| Artifact | Description |
|---|---|
| `wox-mac-arm64.dmg` | Disk image installer (23 MB) |
| `wox-mac-arm64` | Raw executable |

Windows and Linux packages are built through the CI release workflow.

## Building from source

Requirements: Go, Node.js + pnpm, uv, and platform tooling (see the `Makefile` `_check_deps` target).

```bash
make build                 # full build + package for the current platform
make -C wox.core build     # build the core binary into release/
```

## Documentation & community

- Upstream project: [Wox-launcher/Wox](https://github.com/Wox-launcher/Wox)
- Official site: [woxlauncher.com](https://www.woxlauncher.com/)

## License

[GPL-3.0](LICENSE) — fork of [Wox](https://github.com/Wox-launcher/Wox).

---

# 简体中文

**Wox** 是一款免费、开源的**跨平台启动器**，支持 Windows、macOS 和 Linux，是 Alfred、Launchy、Raycast 和 PowerToys Run 的快速键盘流替代品。按下快捷键唤起，输入即搜，即可在应用、文件、文件夹、浏览器书签、网页等之间快速查找。它拥有丰富的插件与主题生态，可以用 JavaScript、Python、C# 等语言扩展。

本仓库是 [Wox](https://github.com/Wox-launcher/Wox) 的一个**分支（fork）**，带有一套经过裁剪的改动，详见[本分支的改动](#本分支的改动)。

## 主要特性

- **搜索一切** —— 应用、文件夹、文件、浏览器书签、网页搜索等，支持中文程序名的模糊与拼音匹配。
- **插件与主题** —— 从商店安装插件和主题，也可以自己开发；支持 JavaScript、Python、C# 等语言。
- **快速、免安装** —— 无需安装，单个可执行文件即可运行。
- **可扩展的工作流** —— 系统插件覆盖剪贴板、笔记、截图、快速跳转、窗口管理等功能。

## 本分支的改动

与上游 [Wox](https://github.com/Wox-launcher/Wox) 相比，本分支：

- 停用了颜色、计算器、计时器、表情符号、笔记、快速跳转、速览、划词、更新、听写、提醒、反馈、单位转换、菜单、主题和云同步。代码还在，只是不再注册。
- 插件指示器、插件安装器和自动化测试插件仍会加载，但已安装插件列表里不显示。
- 关闭了自动更新，设置里也不再提供更新入口。云同步入口同样去掉了。
- 备份与日志里、现有日志下面增加了同步。导出把当前全部设置存成一个文件，导入这个文件会恢复这些设置。
- 系统命令只输入触发词时，结果里列出当前可用的子命令。还要继续输入的命令，回车后把查询补到下一级，再等待参数。
- 插件管理同样只输入触发词就会列出安装、更新、卸载、创建，以及本地插件的相关命令。回车后进入对应命令。
- Shell 只在 `>` 后面还没输入时显示历史。一开始打命令，结果里就只保留要执行的内容。
- 文件夹插件里按 Tab，会把当前选中的目录补进查询，并继续列出下一层。
- 以 `http://` 或 `https://` 开头的内容直接识别为网址，中文域名也可以。
- 新增本机信息插件，触发词是 `info`。先列出本机、IP、CPU、内存、网络、能耗、磁盘和用户。回车进入明细，明细项回车后复制。没有能耗数据的系统不显示能耗。
- 媒体播放器读不到正在播放的组件时，结果里提供安装。设置里在关键词上面显示安装状态，macOS 和 Windows 可以从这里安装。

## 版本与发布

当前版本：**v0.0.1**

`release/` 目录下已提供 **macOS（Apple Silicon，arm64）** 预编译包：

| 构件 | 说明 |
|---|---|
| `wox-mac-arm64.dmg` | 磁盘映像安装包（23 MB） |
| `wox-mac-arm64` | 原始可执行文件 |

Windows 与 Linux 安装包通过 CI 发布工作流构建。

## 从源码构建

依赖：Go、Node.js + pnpm、uv 以及平台工具（见 `Makefile` 的 `_check_deps` 目标）。

```bash
make build                 # 完整构建并打包当前平台
make -C wox.core build     # 只构建核心二进制到 release/
```

## 文档与社区

- 上游项目：[Wox-launcher/Wox](https://github.com/Wox-launcher/Wox)
- 官网：[woxlauncher.com](https://www.woxlauncher.com/)

## 许可证

[GPL-3.0](LICENSE) —— [Wox](https://github.com/Wox-launcher/Wox) 的分支。
