---
name: Yotta extension panels
description: Scoped record of independent panel management, workflow references and floating operation.
typography:
  title:
    fontSize: '18px'
    fontWeight: 600
  value:
    fontSize: '16px'
    fontWeight: 500
  label:
    fontSize: '12px'
rounded:
  log: '8px'
spacing:
  content: '20px'
  grid: '20px'
  group: '16px'
  log: '12px'
---

# Design System: Yotta extension panels

## Overview

This scoped record covers `PanelRenderer.vue`, `PanelControl.vue`, `PanelLog.vue`, `../../views/PanelManagerView.vue`, `../../views/tools/ExtensionPanelsView.vue`, `../../app/editor/PanelNodeField.vue`, `../../app/editor/WorkflowPanelDefault.vue` and its placement in `../../app/editor/WorkflowEditorCanvas.vue`. Inherit Yotta's established dark Nuxt UI and Operate mode; shared HUD, inspector and application tokens remain authoritative. The manager retains SettingsSection and SettingsView.css and reuses the shared inline arrangement table in `../arrangement/`, also used by `../../views/SettingsLauncher.vue`. Launcher reuse is scoped evidence, not a new global design rule.

Panels are independent resources created in the manager or supplied by plugins. Workflows select and use them through typed panel/component references. A workflow finishing ends its waiting subscription, not the panel or its controls. This replaces the former run-owned-tab and ended-read-only design record.

Current table evidence at the repository root: `.task/table-arrangement/{launcher,panels,panels-narrow,live}.png`. The refreshed live capture shows ready readings of 42. Reopen verification in `.task/table-arrangement/reopen.cjs` confirmed persisted order, star icons and initial values of 42, then waited for visible readings before recapturing the window. Toolbar and override evidence remains `.task/panel-layout/{default-menu,node-override}.png`. Event-triggered new runs and arbitrary custom renderers are not established by these screens.

## Colors

Use existing semantic roles. Primary marks selected panels, active tabs, pin state and valid readings. Highlighted text carries names and values; muted text carries labels, units, update times and writer state. Neutral badges describe panel session status. Warning and error roles communicate unavailable selections and failures. Default borders divide sections; logs use the sunken surface. Do not introduce panel-specific color literals.

The latest writer's completion is muted explanatory text, separate from the panel's ongoing live status. Waiting information appears in text; where a waiting dot is shown, it supplements a textual status.

## Typography

Inherit application fonts. Titles and readings use the frontmatter roles; tabular numerals keep updates steady. Labels and units remain subordinate. Format numbers by locale and configured precision, and show an em dash for missing readings. Manager section titles use medium-weight body text, keeping editable names more prominent than component kinds.

## Layout

The manager has a compact title/action header followed by a panel catalog and settings page. At the medium breakpoint, the catalog occupies 220px beside the flexible settings area, with independent vertical scrolling. Below that breakpoint, the catalog stacks above the settings in a shared scrolling area. Catalog padding is 16px; settings spacing follows SettingsView.css. Basic settings and components occupy separate SettingsSection surfaces. Components occupy a compact shared table with fixed column roles: selection, drag grip, order, icon, name, type, initial value, ID and More. Edit fields inline. At narrow widths the table scrolls horizontally within its rounded border (panel minimum width 740px), preserving columns instead of becoming stacked editors. A selection toolbar replaces the column headers in the same 40px space; preview remains a separate settings card, expanded by default.

The floating HUD fills its native resizable window. Header, horizontally scrollable tabs and footer remain outside the vertically scrolling body. Content uses the existing frontmatter spacing, with two columns at the small breakpoint and one below it; groups and logs span both columns. Titles, status and actions wrap naturally instead of shrinking to fit.

## Elevation & Depth

Retain the HUD's subtle top tint and inset border. The manager uses the established settings section surfaces and bordered collections, with thin separators between table rows. The floating body remains mostly flat. Elevated catalog selection and the sunken log area convey hierarchy. Shared Nuxt UI components own focus, hover and pending treatments; no new animation or shadow vocabulary is introduced.

## Shapes

Reuse shared control shapes and the existing rounded log region. Catalog entries have gently rounded selection backgrounds. Reuse SettingsSection outlines and rounded surfaces for the manager. Thin table separators organize component rows; floating value groups keep open separators without enclosing each reading in a card.

## Components

- **Panel catalog:** separate personal and plugin sections, with named selectable entries and explicit empty text. Duplicate names gain short identifier suffixes. Preserve selection during catalog refreshes.
- **Manager actions:** create, show, save, discard and delete sit near the relevant heading. Save reflects dirty/busy state. Plugin definitions are read-only in the manager, while their live controls retain provider-defined behavior.
- **Definition editor:** configure numbers, text, toggles, inputs, dropdowns, buttons, logs, progress and timers through compact inline rows. Names and icons are directly editable; type stays a muted label. Initial values use the matching input, switch or selector, while buttons/logs show an em dash. Dropdown option lines open in a small cell popover. IDs use a compact reference/copy cell. Definitions belong in the manager, not workflow nodes. The shared launcher table uses the same editing grammar with a workflow column instead of panel initial values.
- **Selection and arrangement:** checkboxes select individual rows or all rows with an indeterminate header state. Selected rows use a subtle primary tint. Dragging a selected row moves the selected group while preserving its relative order. The selection toolbar replaces the visible column headers in the same 40px space, retaining column geometry and row positions. It offers select all, bulk common-field editing, destination-based movement, More and clear selection. Row More contains delete and move-to-top/bottom actions. Grip keyboard shortcuts (Alt+ArrowUp/Down) provide step movement without permanent up/down buttons. Keep names, icons and values accessible without expanding rows.
- **Preview:** use a settings card beneath configuration, initially expanded for both personal and plugin panels. Collapsing hides only the contents and retains the card header. Reuse the typed renderer. It previews the definition without dispatching behavior; do not treat preview interaction as a live panel action.
- **Workflow default:** an always-visible icon button in the canvas toolbar, immediately right of the automation target, opens a compact popover. It is available even on a blank workflow; no conditional top bar is needed. The popover offers default selection, show, clear and management, refreshing its catalog on open. It selects an existing resource. The legacy Use panel node is hidden from new-node menus and is not a prerequisite; existing workflows remain compatible.
- **Node references:** nodes inherit the workflow default initially, showing an inheritance badge and resolved name. Checking “单独指定面板” reveals the panel override selector; unchecking restores inheritance. Compatible typed components use labeled searchable selectors. Show panel/component identity beneath the reference when available. Connected references disable manual selection and explain that the connection supplies the value. Keep missing references visible as missing selections; show a warning when no compatible component exists and a link to management.
- **Floating tabs and identity:** ghost tabs use primary selection, tab semantics and Arrow/Home/End keyboard navigation. Display panel identity rather than presenting each workflow execution as a newly owned tab. Keep owner information in the footer.
- **Readings and lifecycle:** values, validity, timers and progress share the renderer. Last update and writer status are distinct from panel status. Preserve the last values/logs when a workflow finishes; the independent panel remains usable.
- **Live controls:** labeled dropdowns, toggles and inputs preserve focused drafts; text inputs have Apply. For managed panels, enable buttons only while a matching workflow subscription waits (or a subscription waits for any component). With no waiting workflow, disable buttons and explain why, while switches and inputs remain usable. Provider-owned controls follow provider behavior. Pending actions and unavailable panel sessions still disable affected interaction and report failures.
- **Logs:** bounded scrolling, timestamps and wrapped text; show the latest 200 records with truncation disclosure. Preserve reader position away from the bottom and offer Follow.
- **Window chrome:** draggable header, pin and hide actions. Hiding the window does not stop running tasks or end the panel. Source errors and truly ended/unavailable provider sessions remain distinct from a workflow writer finishing.

## Do's and Don'ts

- Do preserve the existing dark Operate hierarchy, semantic colors, keyboard focus and localized labels.
- Do keep ownership, latest writer status and waiting subscriptions visibly distinct.
- Do author panel definitions in the manager and expose typed usage references in workflows.
- Do reuse settings sections and the launcher's shared inline table; preserve stable columns and selected-row order during movement.
- Do keep the default-panel toolbar entry available on blank workflows and make node overrides explicit.
- Do preserve panel values and editable controls after workflow completion.
- Do explain disabled managed buttons without disabling unrelated settings.
- Don't restore run-owned tab creation, workflow-node component registration or blanket read-only behavior on workflow completion.
- Don't restore a conditional default-panel top bar or require a Use panel node before ordinary panel operations.
- Don't reintroduce row accordions, Edit/Close expansion or permanent up/down buttons for ordinary table editing.
- Don't dispatch live behavior from the manager preview.
- Don't turn high-frequency readings into log rows or move layout on every update.
- Don't imply that current management includes event-triggered new runs or arbitrary custom rendering.

The layout preview opens by default in its own settings card. Collapsing hides only its contents, retaining the card and its preview toggle. Selection geometry and both preview states were verified in .task/table-arrangement/header.cjs.
