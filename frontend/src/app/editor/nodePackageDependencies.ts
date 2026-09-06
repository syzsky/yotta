import type {
  YottaWorkflowSource,
  NodePackageDependency,
} from '../../../../contracts/workflow/current/workflow-source'

export function syncNodePackageDependencies(
  source: YottaWorkflowSource,
  packages: NodePackageDependency[],
): void {
  for (const graph of source.graphs)
    for (const node of graph.nodes) {
      const dependency = packages.find((p) =>
        p.nodeRefs.some(
          (ref) =>
            ref.nodeTypeId === node.nodeRef.nodeTypeId &&
            ref.semanticDigest === node.nodeRef.semanticDigest &&
            ref.version === node.nodeRef.version,
        ),
      )
      if (!dependency) continue
      const current = source.dependencies.find((item) => item.packageId === dependency.packageId)
      if (current) {
        if (current.manifestDigest !== dependency.manifestDigest) continue
        if (
          !current.nodeRefs.some(
            (ref) =>
              ref.nodeTypeId === node.nodeRef.nodeTypeId &&
              ref.version === node.nodeRef.version &&
              ref.semanticDigest === node.nodeRef.semanticDigest,
          )
        )
          current.nodeRefs.push({ ...node.nodeRef })
      } else source.dependencies.push({ ...dependency, nodeRefs: [{ ...node.nodeRef }] })
    }
}
