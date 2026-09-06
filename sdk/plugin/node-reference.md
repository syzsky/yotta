# Yotta plugin node reference

A package contributes immutable Node Contracts. Node identity is the stable nodeTypeId plus an explicit SemVer version and semanticDigest. The runtime never derives a node name from the Yotta application version.

Each implementation is pinned by packageId, manifest artifactDigest, ABI kind/version, and entrypoint. Process v1 payloads are Windows PE executables; WIT v1 payloads are application/wasm modules executed by the trusted runner without WASI.

Guests receive canonical Value Envelopes and must return the exact declared output ports. Resource, state, entropy, wait, status, and action operations cross the mediated protocol; filesystem paths, credentials, native handles, frontend JavaScript, Vue, and DOM access are not plugin APIs.

Configured targets use the same resource exchange with the declared target ID
as requirement_id. Declare host feature
https://schemas.yotta.dev/host-features/plugin-configured-targets/v1 and use the
Go SDK's OpenTarget: its open config is { "kind": "...", "config": {} }.
The host resolves the local slot from the contract's slotConfigKey; subsequent
Invoke/Drop calls use the same target ID and returned handle. Capability sessions
retain their existing open-config representation.
