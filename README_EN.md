<p align="center">
  <img src="build/appicon.svg" width="112" height="112" alt="Yotta icon">
</p>

<h1 align="center">Yotta</h1>

<p align="center">
  Local-first visual automation<br>
  Connect windows, input, images, Android, browsers, networks, and AI with typed nodes
</p>

<p align="center"><a href="README.md">中文</a></p>

[![CI](https://github.com/yuelioi/yotta/actions/workflows/ci.yml/badge.svg)](https://github.com/yuelioi/yotta/actions/workflows/ci.yml)
[![GitHub release](https://img.shields.io/github/v/release/yuelioi/yotta)](https://github.com/yuelioi/yotta/releases)

Yotta turns repetitive operations into visual workflows. Add nodes, connect execution paths, select
targets and resources, then run, debug, reuse, or share the result without expressing every task as
a script.

![Yotta workflow editor](https://raw.githubusercontent.com/yottaapp/docs/main/content/assets/workflow-editor.png)

## What you can do

- Build typed visual workflows with branches, loops, subgraphs, variables, and annotations.
- Automate Windows applications with pointer, keyboard, window activation, and capture nodes.
- Create editable input macros or precise recordings with continuous movement and dragging.
- Connect Android ADB devices, Chrome/Edge pages, HTTP services, local apps, and AI models.
- Inspect durable Run timelines and debug with breakpoints and step controls.
- Launch frequent workflows from a floating launcher, hotkeys, or schedules.

## Workflows and editing

The workflow library supports search, categories, tags, sorting, import, and export. The editor keeps
node creation, typed connections, configuration, validation, execution, and debugging in one place.

![Workflow library](https://raw.githubusercontent.com/yottaapp/docs/main/content/assets/workflows.png)

Portable workflows keep logical target slots instead of machine-specific window identities. After
importing a workflow on another computer, bind its slots to targets configured on that device.

## Input recordings

Macros are suitable for editable key presses, clicks, scrolling, and waits. Precise recordings retain
continuous pointer movement, drag paths, relative camera motion, and original timing.

<table>
  <tr>
    <td><img src="https://raw.githubusercontent.com/yottaapp/docs/main/content/assets/macro-editor.png" alt="Macro editor"></td>
    <td><img src="https://raw.githubusercontent.com/yottaapp/docs/main/content/assets/precise-recording.png" alt="Precise recording editor"></td>
  </tr>
  <tr>
    <td align="center">Editable macro</td>
    <td align="center">Precise recording</td>
  </tr>
</table>

## Download and install

The current version is **4.0.0-alpha.2**, with full support focused on **Windows 11 x64**.

1. Download the Windows package from [GitHub Releases](https://github.com/yuelioi/yotta/releases).
2. Extract it to a permanent directory.
3. Run `Yotta.exe`.

> This is an alpha release. Export backups of important workflows before upgrading. Linux and macOS
> hosts are currently preview-grade.

## Your first automation

Start with a safe desktop click: add a desktop application, create a Windows automation target,
create a workflow, select its default target, then connect Run Started to Click Pointer. Follow the
[step-by-step guide](https://github.com/yottaapp/docs/blob/main/content/en/getting-started/index.md).

![Yotta settings](https://raw.githubusercontent.com/yottaapp/docs/main/content/assets/settings.png)

## Documentation

Read the complete [Yotta User Guide](https://github.com/yottaapp/docs/blob/main/content/en/index.md).

- [Getting started](https://github.com/yottaapp/docs/blob/main/content/en/getting-started/index.md)
- [Workflow editor](https://github.com/yottaapp/docs/blob/main/content/en/workflow-editor/index.md)
- [Node basics](https://github.com/yottaapp/docs/blob/main/content/en/nodes/index.md)
- [Automation targets](https://github.com/yottaapp/docs/blob/main/content/en/automation/index.md)
- [Input recordings](https://github.com/yottaapp/docs/blob/main/content/en/resources/index.md)
- [Floating launcher](https://github.com/yottaapp/docs/blob/main/content/en/launcher/index.md)
- [Settings](https://github.com/yottaapp/docs/blob/main/content/en/settings/index.md)
- [Runs and debugging](https://github.com/yottaapp/docs/blob/main/content/en/runs/index.md)
- [Schedules](https://github.com/yottaapp/docs/blob/main/content/en/schedules/index.md)
- [Shortcuts](https://github.com/yottaapp/docs/blob/main/content/en/shortcuts/index.md)
- [Updates and backups](https://github.com/yottaapp/docs/blob/main/content/en/maintenance/index.md)
- [Troubleshooting](https://github.com/yottaapp/docs/blob/main/content/en/troubleshooting/index.md)

## Data, privacy, and license

Workflows, settings, resources, and Run history are stored locally. Yotta contacts network services
or AI providers only when you configure and run the corresponding features.

Yotta is **source-available**, not OSI open source. See [LICENSE](LICENSE) for personal, educational,
research, commercial, and redistribution terms.

## Development

- [Contributing](CONTRIBUTING.md)
- [Security policy](SECURITY.md)
- [Developer documentation](docs/README.md)
- [Architecture](docs/architecture/README.md)
- [Release process](RELEASING.md)

For source builds, treat [`go.mod`](go.mod), [`frontend/package.json`](frontend/package.json), and
[`Taskfile.yml`](Taskfile.yml) as the current toolchain and command authority. Common entry points are
`task dev`, `task check`, and `task build`.
