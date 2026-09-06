# Extension panel contract v1

`panel.Definition` declares a stable panel ID, localized labels, typed state fields and a component tree. `Validate` checks structure at package creation/import. `panel.Contribution` references a declared companion plus snapshot and event paths. Packages with panels use descriptor `yotta.plugin/v2`; existing v1 packages remain readable.

Supported component kinds are `group`, `text`, `number`, `status`, `timer`, `progress`, `log`, `button`, `select`, `toggle`, and `input`. Scalar fields use `string`, `number`, or `boolean`. A select declares its string options; interactive components declare an event name. Component and field IDs are stable within a definition. Labels are plugin-owned locale keys included in descriptor messages. Unknown component kinds are rejected rather than silently omitted.

The host discovers enabled contributions and handles companion startup. A provider exposes:

- GET snapshot: `panel.Snapshot`, protocol `yotta.panel-provider/v1`, new session ID for every provider lifetime, monotonically increasing revision, status, values, control revisions, and bounded named record lists.
- POST event: `panel.Event` with session/event/component IDs, event name, the control revision and typed value. Reply with `panel.Result` echoing the event ID and current snapshot. HTTP 409 reports a changed session/control. Actions are never automatically retried after an uncertain transport result.

Current statuses: ready, waiting, stale, unavailable, ended. Number widgets use optional display precision. Status widgets bind a boolean. Timer values are the provider session's Unix-millisecond start instant; the view formats elapsed time. Logs carry IDs/timestamps and text or message keys. Providers must document their retention; the renderer shows the last 200 records, preserving manual scrolling.

Data refresh revisions and control revisions have different meanings. Telemetry does not invalidate a user's selection. Programmatic snapshots do not generate input events. Provider implementations own deduplication of recent event IDs; receiving the same ID with a different request is a conflict. The host validates input types before dispatch and rejects replies from a different provider session.

Panel definitions and provider sessions outlive individual node invocations. Windows may subscribe by repeatedly reading current state, then stop reads when hidden; background recording and workflow consumers can continue to retain the provider. The host never stops a companion simply because its panel window closed. Source failure retains the last view and disables interactions until a successful read.

## Independently managed panels

Components may specify an optional `icon` using a bundled `i-tabler-*` icon name, for example `i-tabler-map-pin`. The manager edits icons inline and the floating panel renders them alongside component labels. Omitting the icon preserves the existing text-only presentation.

Panels are configured in the panel manager or registered by an installed plugin. Workflow nodes cannot create panel definitions or register components. User definitions are saved separately from workflow sources; automatically assigned panel/component IDs survive renaming and are distinct even when display names match.

The `https://schemas.yotta.dev/nodes/panels/` family selects, shows, reads, writes, appends logs and waits for interactions on existing panels. Workflow `targetDefaults` uses the `panel` target slot; a node can override the selected panel. Optional nominal panel/component data ports carry exact typed references for explicit wiring. The editor offers panel and type-filtered component selectors, without requiring manual IDs.

User panel renames preserve live references and compatible values. Deleting a component or changing its type is checked when it is used; waiting subscriptions notice removed or non-interactive components. Provider-generation changes invalidate older plugin references. Read/write nodes validate the referenced component and field type. Plugin telemetry fields remain provider-owned and read-only to workflows; declared interactive controls can be updated through the provider event protocol. Programmatic writes do not notify workflow interaction listeners.

Each waiting workflow receives its own subscription starting when its wait node executes. A timeout follows the failed route, cancellation releases only that listener, and a shared interaction can notify multiple waiting workflows. Ending a workflow does not delete the panel, disable its controls or stop a plugin provider. User panel values remain in the current application session; reopening Yotta restores the saved definitions and configured initial values.

The retired singular `nodes/panel/` family remains readable for existing sources, but is hidden from the add-node menu and fails with `panels.workflow_creation_retired`. Exact legacy NodeRef migrations preserve source contents and expose an actionable replacement notice; they do not guess how to turn an arbitrary dynamic registration workflow into a persistent definition.
