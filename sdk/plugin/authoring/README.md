# Independent Go plugin authoring

Plugin repositories import the public packages in the pinned Yotta module:

- `sdk/plugin/go`: Process guest protocol, host targets and canonical payloads.
- `sdk/plugin/authoring`: canonical node contracts and the exact built-in scalar types.
- `sdk/plugin/packaging`: signed, complete `.ynp` archives and desktop metadata.

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
