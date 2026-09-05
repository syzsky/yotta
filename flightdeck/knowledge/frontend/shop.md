# Shop desktop surfaces

The market follows a VS Code-style discovery rail and left-aligned detail document, while inheriting Yotta's existing semantic colors and system typography. This is a scoped desktop surface, not a replacement global design system.

- Keep the search and result filters inside a 320px rail (280px on compact desktop widths). Results show real author/version/install state; do not invent ratings or download counts.
- Put the workflow identity, author and install/update/open action together in the detail header. Use actual content for details and release notes. Keep readable content width on large displays.
- In the sub-700px stress layout, reserve height for the result rail; `max-height` alone lets Grid collapse results below the search controls. The native desktop minimum is defined by `mainWindowMinWidth` in `internal/desktopapp/wails_tools.go` (currently 1180px); 900px and 620px captures are extra layout stress checks.
- The account trigger has a 32px avatar frame; its menu uses 48px. Render Identity `picture`, fall back to name initials on absent/failed images, and clear image-failure state when the URL changes.
- Workflow publication uses separate major/minor/patch inputs with one composed version string. The same three-integer format is enforced by Yotta and Registry. Keep malformed input visible with inline explanation and disable publish.
- Publication is split into listing, documentation and version sections. Restore metadata from the previous release before enabling edits; on lookup failure keep publication blocked with retry, rather than silently replacing prior metadata with defaults.
- Reuse IconPicker for public workflow icons. `workflowIcons.ts` loads local 128-icon packs on demand; do not import the full Tabler JSON into an application chunk or rely on a remote icon service for chosen icons.
- Keep summary distinct from detailed Markdown and instructions. Metadata filters and installed-ID scope go to Registry before pagination; frontend-only filtering of a limited page misses valid results. Release-history content remains versioned; author profiles use the verified Identity read model.
- Keep the normal publication form readable at 720px height; reduce default textarea rows and group spacing before increasing dialog height. Retain scrolling for smaller windows and expanded errors.

Implementation: `frontend/src/components/workflow/WorkflowMarketPanel.vue`, `WorkflowVersionInput.vue`, `frontend/src/components/AccountAvatar.vue`, `RegistryAccount.vue`, and the publication section of `frontend/src/views/WorkflowsView.vue`.

Reference: [VS Code Extension Marketplace](https://code.visualstudio.com/docs/configure/extensions/extension-marketplace).
