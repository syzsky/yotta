import { computed, ref, type Ref } from 'vue'
import type { YottaWorkflowSource } from '../../../../contracts/workflow/current/workflow-source'
import type { NodeProjection } from './EditorSession'

export interface WorkflowNodeSearchResult {
  graphId: string
  nodeId: string
  label: string
  icon: string
  searchText: string
}

interface WorkflowNodeSearchOptions {
  source: Ref<YottaWorkflowSource | null>
  nodeProjection: (nodeTypeId: string) => NodeProjection | undefined
  projectionTitle: (projection: NodeProjection) => string
  focusNode: (graphPath: string[], nodeId: string) => Promise<void>
}

export function useWorkflowNodeSearch(options: WorkflowNodeSearchOptions) {
  const open = ref(false)
  const query = ref('')
  const results = computed<WorkflowNodeSearchResult[]>(() => {
    const normalized = query.value.trim().toLocaleLowerCase()
    if (!normalized) return []
    return (options.source.value?.graphs ?? [])
      .flatMap((graph) =>
        graph.nodes.map((node) => {
          const projection = options.nodeProjection(node.nodeRef.nodeTypeId)
          const typeTitle = projection
            ? options.projectionTitle(projection)
            : node.nodeRef.nodeTypeId
          const label = node.label || typeTitle
          return {
            graphId: graph.id,
            nodeId: node.id,
            label,
            icon: projection?.icon ?? 'box',
            searchText: [label, typeTitle, node.id, node.nodeRef.nodeTypeId, graph.id]
              .join(' ')
              .toLocaleLowerCase(),
          }
        }),
      )
      .filter((result) => result.searchText.includes(normalized))
      .sort(
        (left, right) =>
          left.graphId.localeCompare(right.graphId) || left.label.localeCompare(right.label),
      )
      .slice(0, 200)
  })

  function show(): void {
    query.value = ''
    open.value = true
  }

  async function select(result: WorkflowNodeSearchResult): Promise<void> {
    open.value = false
    await options.focusNode([result.graphId], result.nodeId)
  }

  function selectFirst(): void {
    const first = results.value[0]
    if (first) void select(first)
  }

  return { open, query, results, show, select, selectFirst }
}
