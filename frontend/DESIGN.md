---
name: Yotta Frontend
description: Incumbent dark Nuxt UI workspace patterns evidenced by the asset library and path tool.
colors:
  primary: "var(--ui-primary)"
  bg: "oklch(17% 0.012 270)"
  bg-muted: "oklch(21.5% 0.012 270)"
  bg-elevated: "oklch(23% 0.013 270)"
  bg-accented: "oklch(28.5% 0.014 270)"
  border: "oklch(29.5% 0.012 270)"
  border-muted: "oklch(24.5% 0.01 270)"
  border-accented: "oklch(36% 0.014 270)"
  surface: "color-mix(in oklab, var(--ui-bg-elevated) 68%, var(--ui-bg))"
  surface-strong: "color-mix(in oklab, var(--ui-bg-accented) 52%, var(--ui-bg-elevated))"
  text: "var(--ui-text)"
  text-highlighted: "var(--ui-text-highlighted)"
  text-toned: "var(--ui-text-toned)"
  text-muted: "var(--ui-text-muted)"
  text-dimmed: "var(--ui-text-dimmed)"
  action-primary-bg: "color-mix(in oklab, var(--ui-primary) 7%, var(--ui-bg-elevated))"
  action-primary-fg: "color-mix(in oklab, var(--ui-primary) 82%, var(--ui-text-highlighted))"
typography:
  title:
    fontSize: "1.25rem"
    fontWeight: 600
  body:
    fontFamily: "'Segoe UI Variable', 'Segoe UI', 'Microsoft YaHei UI', 'Microsoft YaHei', 'PingFang SC', 'Noto Sans CJK SC', ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.875rem"
    lineHeight: "1.25rem"
  label:
    fontSize: "0.75rem"
    lineHeight: "1rem"
    fontWeight: 500
  field:
    fontSize: "1rem"
    lineHeight: "1.25rem"
rounded:
  sm: "var(--ui-radius)"
  md: "calc(var(--ui-radius) * 1.5)"
  lg: "calc(var(--ui-radius) * 2)"
spacing:
  '2': "0.5rem"
  '3': "0.75rem"
  '4': "1rem"
  '5': "1.25rem"
  '6': "1.5rem"
components:
  button-primary:
    backgroundColor: "{colors.action-primary-bg}"
    textColor: "{colors.action-primary-fg}"
    rounded: "{rounded.md}"
    padding: "0.375rem 0.625rem"
  button-neutral-soft:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.text}"
    rounded: "{rounded.md}"
    padding: "0.375rem 0.625rem"
  button-neutral-ghost:
    backgroundColor: "transparent"
    textColor: "{colors.text}"
    rounded: "{rounded.md}"
    padding: "0.375rem 0.625rem"
  input-outline:
    backgroundColor: "{colors.bg}"
    textColor: "{colors.text-highlighted}"
    typography: "{typography.field}"
    rounded: "{rounded.md}"
    padding: "0.375rem 0.625rem"
  library-navigation:
    textColor: "{colors.text}"
    rounded: "{rounded.md}"
    padding: "0.5rem 0.625rem"
  badge-neutral-soft:
    backgroundColor: "{colors.bg-elevated}"
    textColor: "{colors.text}"
    rounded: "{rounded.md}"
    padding: "0.25rem 0.5rem"
  raised-surface:
    backgroundColor: "{colors.surface}"
---

# Design System: Yotta Frontend

## Overview

**Creative North Star: "Existing shell, library, workarea, inspector"**

Yotta's incumbent interface is a dense, dark Nuxt UI workspace for desktop automation. Its visual structure comes from persistent shell chrome, searchable libraries, task controls and contextual editing. The north star names the existing arrangement; it introduces no new visual world.

This app-level record captures reusable patterns observed in the asset library and current path editor source. Muted surfaces, clear text roles and contained green primary actions keep work legible. The path tool adds a contextual inspector to this grammar; that inspector is not a mandatory column on every screen. The supplied Operate direction describes these sampled task surfaces, not a mode requirement for all future surfaces.

**Key Characteristics:**

- Dark semantic surfaces with thin structural borders.
- Compact system typography and Nuxt UI controls.
- Searchable libraries beside a flexible working region.
- Visible selection, recording, saving and feedback states.

Evidence: [global styles](src/style.css), [path tool](src/views/PathsView.vue), [asset library](src/views/AssetsView.vue), [shell](src/App.vue), [Nuxt UI configuration](vite.config.ts), and [shared modal](src/components/common/BaseModal.vue). [PRODUCT.md](../PRODUCT.md) establishes the existing desktop identity; its extension-panel roadmap is not a global layout prescription. Tokens are normative records of current source, not newly imposed palette or font requirements. Library defaults below are resolved from the locally installed Nuxt UI theme files under `node_modules/.nuxt-ui/ui/`, its `ui.css`, and `node_modules/tailwindcss/theme.css`; those generated files are evidence, not edit targets.

## Colors

Cool, low-chroma dark surfaces carry the interface; semantic green identifies primary work and selected path geometry.

### Primary

**Semantic emerald** (`primary`) is configured by `vite.config.ts`. Primary solid buttons use the shared contained-action foreground and lightly tinted background rather than the unmodified library solid fill. The path line and selected point row also consume the primary role. Amber remains the configured warning family; recording pause and discard controls use warning, while destructive controls and failures use error. These are status roles, not additional brand palettes.

### Neutral

The background family separates canvas, muted regions, elevated controls and accented states. Default, muted and accented border tokens provide boundaries without hardcoded local grays. The mixed `surface` and `surface-strong` roles serve shared workspace and overlay treatments. Nuxt UI's text roles distinguish highlighted headings, normal labels, toned content and muted/dimmed metadata; their semantic CSS bindings remain intact.

**The Semantic Surface Rule.** Reuse the shared background, border and text roles for workspace surfaces and states.

**The Contained Primary Rule.** Use the shared contained primary-solid treatment for the task area's primary action; use neutral soft or ghost controls for supporting actions.

Evidence: `src/style.css` dark scale, shared utilities and primary button states; `vite.config.ts` color and button configuration; both sampled view templates. The sidecar uses an existing emerald ramp for primary metadata. No synthetic ramps are assigned to mixed surfaces or semantic text bindings.

## Typography

**Body Font:** the existing Windows-first system stack recorded in the frontmatter, including Chinese fallbacks. It is assigned to `html` and `body` in `src/style.css` and inherited by headings and controls.

**Label/Mono Font:** asset metadata uses the installed Tailwind monospace stack for identifiers and resolution readouts. Importing JetBrains Mono in the global stylesheet does not establish it as the body or metadata font; no new font assignment is recorded here.

The hierarchy is compact and task-oriented. Assets uses the title size and semibold weight with tight leading and slight negative tracking. Paths now inherits the compact title treatment from the shared HudShell. Body copy and point rows use the body role, supporting labels use the smaller label role, and default medium Nuxt UI inputs use the field role. There is no evidenced display/hero type role. Small asset counters and category captions remain metadata-specific rather than a new body-size standard.

**The Numeric Scan Rule.** Preserve tabular numerals for ordered point indices and coordinate columns, and use the incumbent monospace treatment for technical asset metadata.

Evidence: page headings, point rows and form fields in `PathsView.vue`; category navigation and macro metadata in `AssetsView.vue`; local Nuxt UI input/button size definitions and Tailwind's text scale. The title token remains unchanged; the path editor inherits its heading treatment from `HudShell.vue` rather than defining a new global title role.

## Layout

The application shell fills the desktop viewport and supplies title/status chrome. Routed workspaces use full-height flex layouts, shrinkable working regions and explicit inner scrolling. A page header establishes identity and actions; the library provides search or resource categories; the flexible workarea owns the current task. Borders divide toolbar, list and footer regions. Labels and names truncate where the source permits; controls and numeric values retain their working space.

The recurring spacing rhythm comes from Tailwind's quarter-rem base, with the recorded steps used for control gaps, form groups and panel padding. Header padding varies by route. Nested form groups use two columns where appropriate. The path editor extends the incumbent screenshot-editor structure through `HudShell` and `usePickerViewport`: the left workarea contains a toolbar, flexible canvas and fixed bottom waypoint tray; the right inspector has scrollable position-source, selected-point and resource-metadata sections above a fixed save footer. Selection connects the waypoint tray and canvas to the selected-point controls. At compact widths the inspector stays on the right, while its container query stacks the X/Y fields; it does not turn the editor into a stacked page form. The shared asset library owns path listing and filtering; the editor also saves name, category and tags through the shared asset APIs. The path detail editor has no duplicate library sidebar. Assets wraps its header at its own threshold but retains its category rail; this is not evidence of a universal mobile layout.

**The Working Region Rule.** Give task content a shrinkable, scrollable working region while keeping its surrounding actions and navigation structurally distinct.

Evidence: `App.vue` shell and scroll gutter; both route templates' flex/grid, overflow, border and spacing utilities. Route-specific breakpoints, rail widths and path preview dimensions are intentionally not promoted to global tokens.

## Elevation & Depth

Depth is primarily tonal, with thin borders separating adjacent regions. Shared raised surfaces pair the mixed surface color with a default border. Overlays and toasts additionally use real shadows, and primary buttons use inset border/highlight shadows with small outer shadows. The workspace canvas includes a subtle primary radial wash; the asset library uses it and the path page does not. Both belong to the incumbent system, so neither gradients nor shadows are globally prohibited.

The sidecar records the shared overlay shadow and primary-button motion/shadow treatment directly from `src/style.css`. Overlay elevation is not a resting shadow requirement for every panel. Hover, active, focus-visible and disabled primary states remain the shared implementation, including its short color/background/shadow transition.

## Shapes

Controls use the Nuxt UI small-radius family. Its current base radius is a quarter rem; the recorded medium and large formulas resolve to the incumbent control and grouped-container corners. Repeated grouped regions in Assets use the large radius. The path canvas and waypoint tray are divided by structural borders, while its inspector sections use a local rounded treatment; this does not change the global radius tokens. List rows remain flat and border-divided. Badges use the library's size-dependent radius. Rounded recording indicators are status marks, not a pill-shaped default for all controls.

Evidence: local Nuxt UI radius bindings and button/input/badge themes; `AssetsView.vue` grouped editor metadata and status indicator; `PathsView.vue` preview and point list.

## Components

### Buttons

Compact, contained task controls. Primary solid uses the app override; neutral soft provides a visible supporting surface and neutral ghost keeps secondary row actions quiet. Default medium buttons share the frontmatter padding and medium radius. The app centers button content; library navigation explicitly aligns it left. Shared primary hover and active states adjust the tint and shadow, focus-visible adds an offset outline, and disabled states mute the treatment. Busy actions expose loading, while recording disables conflicting edits in the path tool. Error-colored delete controls retain their destructive meaning.

### Chips

Compact metadata, not decorative labels. The asset library uses neutral soft badges for counts and semantic soft badges for recording state. They are noninteractive text indicators; no invented hover or focus behavior is added. Frontmatter and sidecar show the installed medium neutral-soft primitive; compact size variants stay owned by Nuxt UI.

### Cards / Containers

Flat grouped work. Shared raised surfaces use the mixed surface background and a thin default border. Workspace bands often have only an adjoining divider; grouped editing panels add the large radius and internal spacing. Reuse the appropriate incumbent container treatment rather than assuming every region needs a card. The sidecar's raised-surface sample records only the shared utility, without invented padding or corner rules.

### Inputs / Fields

Nuxt UI inputs sit in labeled form fields or have explicit accessible names for search and compact controls. The default outlined input uses highlighted text over the default background, an inset accented border and semantic primary focus treatment. Placeholder text is dimmed; disabled controls retain the library's cursor/opacity treatment. The global wrapper establishes full width while allowing explicit width utilities. Numeric editing uses `UInputNumber`; field descriptions explain units and sampling behavior. Errors surface through alerts or status feedback already present in the views.

### Navigation

Library navigation uses left-aligned neutral buttons: soft for the active item/category and ghost otherwise. Point rows additionally combine elevated selection background, primary text and pressed state. Paths is a peer of macros, precise traces and templates in the library category rail, using the map-route icon. Its detail editor returns to the path category. The workflow rail exposes saved paths; resource-name clicks bind a compatible selected node or otherwise insert the default execution node. The shell, local library and selected-object controls are separate navigation contexts; the sampled source does not establish a universal mobile menu.

### Workarea and inspector

Selection determines the detail controls near the working artifact. In Paths, choose a configured network profile as the position source, or expand the advanced manual sampling-URL field; record or mark points, edit the ordered list and selected coordinates, then save the path and its resource metadata. Saved paths can subsequently be used in the workflow through Follow Saved Path, or Read Path for data transformations; the path editor itself has no node insertion control. Assets independently pairs recording, editing and saving within the library and shared modals. These examples establish task continuity without requiring other tools to duplicate the path workflow.

The path inspector displays the current `paths.mark` binding and links to the hotkey center. This persistent recording entry defaults to F6; it is registered through `RegisterRecording` in `internal/desktopapp/desktop.go`, and central `OnSystemChange` persists changes to `settings.UI.PathMarkHotkey`. Only the active editor handles mark events. Leaving the editor removes its listener, not the registered shortcut. There is no local Ctrl+Shift+F6 enable input or Ctrl+Enter mark handler.

Pause stops sampling while keeping the recording session available to continue. The separate neutral-soft Finish Recording action is visible during an active or paused session; it stops sampling, clears the session/continuity state, and retains all recorded points for editing and explicit saving. It neither discards points nor saves automatically.

### Resource use and drag

Resource rows separate the neutral name control from the contained green soft Use action. Dragging into the canvas inserts the common execution node at the drop position: Follow Saved Path for paths, Click Template for templates, and the corresponding playback node for macros or precise traces. A drop does not replace the selected node's binding. Green Use offers explicit alternatives when there are several actions: Follow Saved Path / Read Path, or Click Template / Wait for Template / Wait for Template Gone. A single available action executes the insertion directly without a one-item menu. These are authoring actions, not immediate automation runs. Clicking the resource name preserves the existing compatible-selected-node binding behavior; without a compatible selection it inserts the default node. Path editing stays in the path tool, while node insertion stays in the workflow editor.

### Variable types and position-source setup

Path is a state-variable type as well as a node data type. Variable creation and type editing share the searchable `TypeSelect`; string, boolean, number, integer, JSON, path, world position and key code precede the remaining eligible types. The full eligible catalogue remains searchable: durable types with declared initial values, plus the existing key-chord choice. Selected and menu-item color dots consume `projection.color` through the shared choice data, with the existing neutral fallback only when absent. Preserve the same projection-derived variable-list colors; do not introduce a separate type-to-color table or new palette tokens.

The position-source picker lists configured network HTTP profiles by label and derives their sampling endpoint. The path inspector exposes manual URL entry under an advanced disclosure. The movement-node setup uses the same picker in profiles-only mode: its string-variable field explains the required complete position-source sample and shows guidance when no compatible variable exists. The setup disclosure opens by default for an unbound field; setup requires a selected profile, checks a sample first, and reports failures inline.

Successful setup creates the sampling nodes, an initially empty string variable and an initially true `.active` boolean variable, and binds the movement node in one `applyBatch` / one undo step. Generated names avoid existing variable collisions. Sampling starts with the run at a fixed 100 ms interval. Each periodic tick reads active state: true samples HTTP and writes the response body; false stops the periodic task from its internal branch. The movement node's execution outcomes write active=false; they must not connect directly across execution regions to periodic stop. This interval describes generated workflow sampling, not the path recorder's sampling clock.

### Verification boundary

The existing browser journey result records actual Vue components with mock transport covering resource actions, path drag, source selection, type colors, pause, finish and save, with no reported errors. The current work handoff also records successful real CLI validation producing a compiled Program for the generated sampling workflow. Simulated UI interaction and CLI compilation are separate evidence: neither demonstrates live movement or a newly launched desktop UI. This documentation-only pass checks source and existing evidence; it does not rerun builds or checks, launch/restart the desktop App, or claim real-game or native in-game hotkey validation. Map/minimap localization remains unimplemented, and the coordinate canvas is not a game map. The existing visual world and token frontmatter are unchanged; the sidecar is outside this handoff's write scope.

Evidence: `src/app/editor/WorkflowPathDock.vue`, `WorkflowResourceDock.vue`, `resourceNodeActions.ts`, `useWorkflowResourceAuthoring.ts`, `positionSourceAuthoring.ts`, `PositionSourceField.vue`, `stateVariableTypes.ts`, `WorkflowStatePanel.vue`; `src/app/paths/PositionSourcePicker.vue`; `src/components/common/TypeSelect.vue`; `../../workspace/.task/path-tools-ui/journey-result.json` and `../../workspace/flightdeck/work/path-tools/index.md` (existing verification evidence, not implementation authority); `PathsView.vue` template and selection/save state; `src/components/tools/HudShell.vue`; `src/composables/tools/usePickerViewport.ts`; `../internal/desktopapp/desktop.go` and `../internal/services/settings.go`; `src/i18n/locales/en/resources.ts` path copy; `AssetsView.vue` recording and modal controls; `BaseModal.vue` header/body/footer structure. Sidecar snippets are source-derived static HTML/CSS samples of existing primitives; they do not simulate application state or execute Vue behavior.

## Do's and Don'ts

### Do:

- Do reuse semantic surface, text and state roles from the existing theme.
- Do keep primary, supporting and destructive actions visually distinguishable.
- Do preserve selection, keyboard focus, accessible labels and visible busy/recording feedback.
- Do keep library navigation, working content and contextual editing legible as separate regions.

### Don't:

- Don't replace the existing system font stack or introduce a new brand palette for a tool page.
- Don't turn one-off preview sizes, route breakpoints or inspector widths into global design tokens.
- Don't ban the subtle canvas wash or real elevation shadows already present in the shared styles.
- Don't treat the path-to-node handoff as an implemented in-page node creation action.
- Don't replace the path editor's screenshot-editor arrangement with a stacked path form.
