# Independent Go plugin authoring

Create an independent project with `python scripts/create-plugin.py --help` from
the Yotta checkout. The generator uses `sdk/plugin/templates`, accepts an explicit
SDK version and publisher namespace, and optionally adds a tested companion/panel
with `--with-panel`. Generated projects contain README/AGENTS plus `build.ps1`
for Check, Build, Keygen and Pack. It never overwrites an existing project.

The Chinese developer guide is maintained in the public `yottaapp/docs` repository
under `content/zh/plugins/` (overview, quickstart, panels, packaging).

Plugin repositories import the public packages in the pinned Yotta module:

- `sdk/plugin/go`: Process guest protocol, host targets and canonical payloads.
- `sdk/plugin/authoring`: canonical node contracts and the exact built-in scalar types.
- `sdk/plugin/packaging`: signed, complete `.ynp` archives and desktop metadata.
- `sdk/plugin/positionsource`: shared position/heading capability observations;
  see [the position source contract](../positionsource/README.md).

The SDK uses the host's implementation internally. An external plugin imports
only these public packages, never `internal/` or a sibling checkout. Build the
guest and optional companion binaries, then use `packaging.Write` with a
publisher-owned key. Keep that key outside the source tree and retain it for
updates.

For unpublished development work, run `task plugins:export-sdk` in Yotta. It
exports an explicit Go module proxy under `.task/sdk-proxy`, with an immutable
content-based prerelease version. Point the plugin's `GOPROXY` at this artifact
and pin the printed version in go.mod. This is a local distribution artifact,
not a public release or a cross-repository source dependency.

The desktop descriptor is a signed `yotta-plugin.json` payload. It names the
plugin, supplies flat Chinese/English translation keys declared by its node
contracts, and describes optional cooperative loopback companions. Companion
executables and `${package}/` arguments must refer to verified package payloads.
Use a `plugin.<publisher>.*` namespace for node messages; declared node errors
may contribute `error.*` messages. Existing core translations are not replaced.

Companion binaries use the manifest's additional `resources` payloads; every
resource has the same path, size and digest verification as node executables.
Legacy packages without resources or a desktop descriptor remain readable.
Import performs verification and local publisher enrollment in one user action;
updates cannot silently substitute the installed publisher's key.

Settings → Plugins supports import/update, enable/disable, previous-version
restore, uninstall, and companion start/stop. Node catalogs currently refresh
on restart; the page distinguishes persisted enabled state from the current
process's loaded generation. Running workflows retain their package identity
and prevent lifecycle changes until they finish. Workflows survive uninstall.

## Periodic work and live position

For durable ordered geometry, see [path values and nodes](paths.md).

Node contracts use format v3. The host schedules process/Wasm invocations as
bounded asynchronous operations; one plugin invocation must finish or acknowledge
cancellation before its paused branch can hand over input. The guest does not own
a second scheduler and must not run a detached background worker after returning.

Use a Periodic or Monitor task in the workflow to invoke an observation plugin.
Return `authoring.WorldPosition` with output type `authoring.WorldPositionTypeID`,
resolved from `authoring.Builtins()`. Connect that output to State Write using a
variable of the same type. Move Character reads that variable continuously.

`frame` and `unit` identify the coordinate space. `axisHeading` is the reported
heading for positive X, and `axisSign` is +1 or -1 for the coordinate angle
orientation. `epoch` identifies the observation session; change it when the
source restarts or changes coordinate space. `receivedAt` is a trustworthy host
observation time in Unix milliseconds; delayed data must retain its original
age. `sampleAt` and `sequence` retain source metadata. Set `valid=false` when no
current observation exists. A source on an unsynchronized remote clock should
convert its freshness to host time before publishing.

The movement node does not know the plugin identity, HTTP URL or field names.
The built-in Make Position and Parse Position nodes provide the same boundary
for OCR, HTTP JSON and other scalar data sources.
