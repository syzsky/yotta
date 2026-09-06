# Workflow Bundle panels

`Inspect` is the shared, read-only parser used by desktop import and Registry publication. It validates panel definitions and reference coverage without connecting to providers. Workflow Bundle v3 includes sorted `panels` resources in its manifest. Readers accept v1/v2; exports without panels retain v2 encoding.

A managed resource contains a saved definition and initial values, normalized to revision 1. Current readings, logs, runtime generations and installation provenance are excluded. A plugin resource declares the exact publisher, package, version, manifest digest and contributed definition. Public `Info.Dependencies` includes these package requirements even when no nodes from that package appear in the workflow; such entries have an empty `nodeRefs` list.

The desktop collects workflow default panel selections and explicit panel-node selections across all graphs. It validates statically selected components by type. On import it assigns an independent panel identity for each destination workflow/resource pair, rewrites only declared panel selections, and retains component IDs within their panel. Runtime references carried by data edges are produced by the installed workflow, never serialized session handles.

Installation metadata records the previous imported definition. Updating the same destination merges untouched fields from the author and preserves locally edited names, icons, initial values, options and order. Locally edited components removed upstream remain available. Declared components absent locally are restored so their references remain usable. An incompatible merge fails before publishing the workflow. Cloning uses a different destination workflow identity and therefore independent panels. Workflow removal does not delete independently managed panels.

The panel batch is persisted before the workflow. A definite workflow publication failure restores the previous panel batch; a publication whose persisted source matches the intended source is treated as committed. Interrupted attempts reuse deterministic panel identities on retry. Compatible live values and panel references survive definition-only updates.

Plugin definitions remain plugin-owned. Import validates the installed, enabled package and exact contribution without starting its companion. Missing packages report the required plugin/version. Automatic retrieval of missing plugins depends on Node Pack distribution, which the current Registry runtime does not yet implement; the workflow installer does not silently omit those dependencies.

Registry must consume a release of this parser to accept panel-bearing v3 bundles. A desktop update alone does not upgrade a deployed Registry. Local cross-repository verification uses an explicit temporary Go workspace, not a committed source replacement.
