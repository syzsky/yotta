<p align="center">
  <img src="build/appicon.svg" width="112" height="112" alt="Yotta icon">
</p>

<h1 align="center">Yotta</h1>

<p align="center">
  Local-first visual automation<br>
  Connect windows, input, images, Android, browsers, networks, and AI with typed nodes
</p>

<p align="center"><a href="README.md">中文</a></p>

[![CI](https://github.com/yottaapp/yotta/actions/workflows/ci.yml/badge.svg)](https://github.com/yottaapp/yotta/actions/workflows/ci.yml)
[![GitHub release](https://img.shields.io/github/v/release/yottaapp/yotta?include_prereleases)](https://github.com/yottaapp/yotta/releases)

Yotta turns repetitive operations into visual workflows. Add nodes, connect execution paths, select
targets and resources, then run, debug, reuse, or share the result without expressing every task as
a script.

![Yotta workflow editor](https://raw.githubusercontent.com/yottaapp/docs/main/content/assets/workflow-editor.png)

## What you can do

- Build typed visual workflows with branches, loops, subgraphs, variables, annotations, multiple Run Started entries, periodic tasks, and monitoring tasks.
- Automate Windows applications with pointer, keyboard, window activation, and capture nodes.
- Create editable input macros or precise recordings with continuous movement and dragging.
- Record and edit coordinate paths through positioning plugins, then follow them with actions along the route and recovery branches.
- Manage macros, precise recordings, visual templates, and paths in one resource library, with categories, tags, batch operations, and drag-to-create nodes.
- Install node plugins and companion services, with interactive extension panels for information, buttons, and inputs.
- Browse, install, and publish workflows and plugins in the online market, with versions, reviews, and discussions.
- Connect Android ADB devices, Chrome/Edge pages, HTTP services, local apps, and AI models.
- Inspect durable Run timelines and debug with breakpoints and step controls.
- Launch frequent workflows from a floating launcher, hotkeys, or schedules.

## Workflows and editing

The workflow library supports search, categories, tags, sorting, import, and export. The editor keeps
node creation, typed connections, configuration, validation, execution, and debugging in one place.

![Workflow library](https://raw.githubusercontent.com/yottaapp/docs/main/content/assets/workflows.png)

Portable workflows keep logical target slots instead of machine-specific window identities. After
importing a workflow on another computer, bind its slots to targets configured on that device.

A run can have multiple Run Started entries. Use execution connections when order matters. Periodic
tasks support ongoing sampling and checks without queuing overlapping executions of the same task body.

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

## Paths and positioning plugins

Paths are resources alongside macros, precise recordings, and visual templates. Record a route or add
waypoints with a hotkey in a separate, pinnable path window, then edit, name, import, export, and reuse
the points. Drag a path into a workflow to add Follow Saved Path.

Path following uses continuously updated position and heading, with actions during movement, waypoint
actions, and recovery branches. Execution requires a compatible positioning plugin and target turning
calibration. Map/minimap localization and automatic obstacle avoidance are not available yet.
See the [path guide (Chinese)](docs/product/paths.md) and [position source SDK](sdk/plugin/positionsource/README.md).

## Download and install

Yotta is in **4.0 Alpha**, with full support focused on **Windows 11 x64**. Check Releases for the latest
downloadable prerelease and [VERSION](VERSION) for the version of this source checkout.

1. Download the Windows package from [GitHub Releases](https://github.com/yottaapp/yotta/releases).
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
Screenshots here and in the guide come from earlier alpha versions; the current layout may differ.

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
- [Plugin authoring](sdk/plugin/authoring/README.md) and [extension panels](sdk/plugin/panel/README.md)

## Data, privacy, and license

Workflows, settings, resources, and Run history are stored locally. Sign-in, the online market,
publishing, and network nodes contact their respective services. AI features send relevant inputs to
your configured model provider.

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
