import { computed, ref, type Ref } from 'vue'
import type { ConfirmOpts } from '@/composables/useConfirm'
import type { EditorSession } from './EditorSession'
import { projectGraphDefinitions } from './subgraphManagement'
import type { GraphInterfaceCandidateKind, GraphInterfaceItemKind } from './subgraphInterface'

interface WorkflowSubgraphManagementOptions {
  session: EditorSession
  canvasElement: Ref<HTMLElement | null>
  selectedNodeId: Ref<string>
  selectedNodeIds: Ref<Set<string>>
  screenToFlowCoordinate: (position: { x: number; y: number }) => { x: number; y: number }
  clearSelection: () => void
  fitCurrentGraph: () => Promise<unknown> | unknown
  focusNode: (graphPath: string[], nodeId: string) => Promise<void>
  confirm: (options: ConfirmOpts) => Promise<boolean | string>
  translate: (key: string, params?: Record<string, unknown>) => string
  warn: (title: string, description: string) => void
  showError: (title: string, error: unknown) => void
}

export function useWorkflowSubgraphManagement(options: WorkflowSubgraphManagementOptions) {
  const graphDialogOpen = ref(false)
  const graphDialogMode = ref<'create' | 'rename'>('create')
  const graphDialogTargetId = ref('')
  const graphName = ref('')

  const callableGraphs = computed(() =>
    (options.session.source?.graphs ?? []).filter(
      (graph) =>
        graph.kind === 'subgraph' &&
        graph.id !== options.session.currentGraph?.id &&
        !graphReaches(graph.id, options.session.currentGraph?.id ?? ''),
    ),
  )
  const callableGraphIds = computed(() => callableGraphs.value.map((graph) => graph.id))
  const canInferGraphInterface = computed(() => {
    const readiness = options.session.currentGraphInterfaceReadiness()
    if (readiness.valid) return { valid: true, message: '' }
    const key =
      readiness.reason === 'multiple-entry'
        ? 'workflow.graphs.infer_multiple_entries'
        : readiness.reason === 'missing-entry-or-exit'
          ? 'workflow.graphs.infer_missing_endpoints'
          : 'workflow.graphs.infer_not_subgraph'
    return { valid: false, message: options.translate(key) }
  })
  const graphInterfaceCandidates = computed(() =>
    options.session.currentGraph?.kind === 'subgraph'
      ? options.session.currentGraphInterfaceCandidates()
      : [],
  )
  const graphInterfaceReferenceCounts = computed<Record<string, number>>(() => {
    const graph = options.session.currentGraph
    if (!graph || graph.kind !== 'subgraph') return {}
    return Object.fromEntries([
      ...graph.inputs.map((port) => [
        `input:${port.id}`,
        options.session.currentGraphInterfaceReferences('input', port.id).length,
      ]),
      ...graph.outputs.map((port) => [
        `output:${port.id}`,
        options.session.currentGraphInterfaceReferences('output', port.id).length,
      ]),
      ...(graph.exits ?? []).map((exit) => [
        `exit:${exit.id}`,
        options.session.currentGraphInterfaceReferences('exit', exit.id).length,
      ]),
    ])
  })
  const selectedCall = computed(
    () =>
      options.session.currentGraph?.calls?.find(
        (call) => call.id === options.selectedNodeId.value,
      ) ?? null,
  )
  const selectedCallGraph = computed(() =>
    selectedCall.value ? (options.session.calleeGraph(selectedCall.value) ?? null) : null,
  )
  const selectedCallPorts = computed(() =>
    (selectedCallGraph.value?.inputs ?? []).flatMap((port) => {
      const projection = options.session.graphInputProjection(selectedCallGraph.value!.id, port.id)
      return projection ? [projection] : []
    }),
  )

  function graphReaches(graphId: string, targetId: string, visited = new Set<string>()): boolean {
    if (!targetId || visited.has(graphId)) return false
    if (graphId === targetId) return true
    visited.add(graphId)
    const graph = options.session.source?.graphs.find((candidate) => candidate.id === graphId)
    return Boolean(
      graph?.calls?.some((call) => graphReaches(call.graphId, targetId, new Set(visited))),
    )
  }

  function graphLabel(graphId: string): string {
    const graph = options.session.source?.graphs.find((candidate) => candidate.id === graphId)
    return (
      graph?.name || (graph?.kind === 'main' ? options.translate('workflow.graphs.main') : graphId)
    )
  }

  function openCalledGraph(graphId: string): void {
    const entry = options.session.source?.entryGraph
    if (!entry) return
    options.session.openGraphPath(graphId === entry ? [entry] : [entry, graphId])
    options.clearSelection()
    void options.fitCurrentGraph()
  }

  function openGraphAt(index: number): void {
    options.session.openGraphPath(options.session.graphPath.slice(0, index + 1))
    options.clearSelection()
    void options.fitCurrentGraph()
  }

  function openGraphDialog(
    mode: 'create' | 'rename',
    graphId = options.session.currentGraph?.id ?? '',
  ): void {
    graphDialogMode.value = mode
    graphDialogTargetId.value = mode === 'rename' ? graphId : ''
    graphName.value = mode === 'rename' ? graphLabel(graphId) : ''
    graphDialogOpen.value = true
  }

  function commitGraphDialog(): void {
    const name = graphName.value.trim()
    if (!name) return
    try {
      if (graphDialogMode.value === 'create') {
        options.session.createSubgraph(name)
        options.clearSelection()
        void options.fitCurrentGraph()
      } else if (graphDialogTargetId.value) {
        options.session.renameGraph(graphDialogTargetId.value, name)
      }
      graphDialogOpen.value = false
    } catch (error) {
      reject(error)
    }
  }

  async function inferGraphInterface(): Promise<void> {
    try {
      if (!canInferGraphInterface.value.valid) {
        options.warn(
          options.translate('workflow.graphs.infer_interface_blocked'),
          canInferGraphInterface.value.message,
        )
        return
      }
      const preview = options.session.previewCurrentGraphInterfaceInference()
      const referenced = preview.removed.flatMap((item) =>
        item.kind === 'entry'
          ? []
          : options.session.currentGraphInterfaceReferences(item.kind, item.id),
      )
      if (referenced.length) {
        options.warn(
          options.translate('workflow.graphs.infer_interface_blocked'),
          options.translate('workflow.graphs.infer_interface_blocked_hint', {
            count: referenced.length,
          }),
        )
        return
      }
      const accepted = await options.confirm({
        title: options.translate('workflow.graphs.infer_interface_title'),
        description: options.translate('workflow.graphs.infer_interface_preview', {
          added: preview.added.length,
          removed: preview.removed.length,
        }),
        confirmText: options.translate('workflow.graphs.infer_interface_confirm'),
        cancelText: options.translate('common.cancel'),
        color: 'primary',
      })
      if (!accepted) return
      options.session.applyCurrentGraphInterfaceInference(preview)
      void options.fitCurrentGraph()
    } catch (error) {
      reject(error)
    }
  }

  function addGraphInterfaceCandidate(candidateKey: string): void {
    try {
      options.session.addCurrentGraphInterfaceCandidate(candidateKey)
      void options.fitCurrentGraph()
    } catch (error) {
      reject(error)
    }
  }

  function renameGraphInterfaceItem(kind: GraphInterfaceItemKind, id: string, name: string): void {
    try {
      options.session.renameCurrentGraphInterfaceItem(kind, id, name)
    } catch (error) {
      reject(error)
    }
  }

  function moveGraphInterfaceItem(
    kind: GraphInterfaceItemKind,
    id: string,
    direction: -1 | 1,
  ): void {
    try {
      options.session.moveCurrentGraphInterfaceItem(kind, id, direction)
    } catch (error) {
      reject(error)
    }
  }

  function removeGraphInterfaceItem(kind: GraphInterfaceCandidateKind, id: string): void {
    try {
      if (kind !== 'entry') {
        const references = options.session.currentGraphInterfaceReferences(kind, id)
        if (references.length) {
          options.warn(
            options.translate('workflow.graphs.remove_interface_blocked'),
            options.translate('workflow.graphs.interface_referenced', { count: references.length }),
          )
          return
        }
      }
      options.session.removeCurrentGraphInterfaceItem(kind, id)
      void options.fitCurrentGraph()
    } catch (error) {
      reject(error)
    }
  }

  function addGraphCall(graphId: string, requestedPosition?: { x: number; y: number }): void {
    const rect = options.canvasElement.value?.getBoundingClientRect()
    const position =
      requestedPosition ??
      (rect
        ? options.screenToFlowCoordinate({
            x: rect.left + rect.width / 2,
            y: rect.top + rect.height / 2,
          })
        : { x: 160, y: 160 })
    try {
      selectOnly(options.session.insertGraphCall(graphId, position))
    } catch (error) {
      reject(error)
    }
  }

  function duplicateSelectedGraphCall(): void {
    if (!selectedCall.value) return
    try {
      selectOnly(options.session.duplicateCurrentGraphCall(selectedCall.value.id))
    } catch (error) {
      reject(error)
    }
  }

  function forkSelectedGraphCall(): void {
    if (!selectedCall.value) return
    try {
      options.session.forkCurrentGraphCall(selectedCall.value.id)
    } catch (error) {
      reject(error)
    }
  }

  async function expandSelectedGraphCall(): Promise<void> {
    if (!selectedCall.value) return
    const accepted = await options.confirm({
      title: options.translate('workflow.graphs.expand_call_title'),
      description: options.translate('workflow.graphs.expand_call_hint'),
      confirmText: options.translate('workflow.graphs.expand_call'),
      cancelText: options.translate('common.cancel'),
      color: 'primary',
    })
    if (!accepted) return
    try {
      const elementIds = options.session.expandCurrentGraphCall(selectedCall.value.id)
      options.selectedNodeIds.value = new Set(elementIds)
      options.selectedNodeId.value = elementIds[0] ?? ''
      void options.fitCurrentGraph()
    } catch (error) {
      reject(error)
    }
  }

  async function deleteGraphDefinition(graphId: string): Promise<void> {
    const source = options.session.source
    const graph = source?.graphs.find((candidate) => candidate.id === graphId)
    if (!source || !graph || graph.kind !== 'subgraph') return
    const definition = projectGraphDefinitions(source).find((candidate) => candidate.id === graphId)
    if (definition?.callCount) {
      options.warn(
        options.translate('workflow.graphs.delete_definition'),
        options.translate('workflow.graphs.delete_definition_referenced', {
          count: definition.callCount,
        }),
      )
      return
    }
    const accepted = await options.confirm({
      title: options.translate('workflow.graphs.delete_title'),
      description: options.translate('workflow.graphs.delete_hint', { name: graphLabel(graph.id) }),
      confirmText: options.translate('common.delete'),
      color: 'error',
    })
    if (accepted !== true) return
    try {
      options.session.removeGraph(graph.id)
      options.clearSelection()
      void options.fitCurrentGraph()
    } catch (error) {
      reject(error)
    }
  }

  function duplicateGraphDefinition(graphId: string): void {
    try {
      openCalledGraph(options.session.duplicateGraphDefinition(graphId))
    } catch (error) {
      reject(error)
    }
  }

  async function deleteGraphDefinitionCascade(graphId: string): Promise<void> {
    const source = options.session.source
    const graph = source?.graphs.find((candidate) => candidate.id === graphId)
    const definition = source
      ? projectGraphDefinitions(source).find((candidate) => candidate.id === graphId)
      : undefined
    if (!source || !graph || graph.kind !== 'subgraph' || !definition?.callCount) return
    const accepted = await options.confirm({
      title: options.translate('workflow.graphs.delete_definition_cascade_title'),
      description: options.translate('workflow.graphs.delete_definition_cascade_confirm', {
        name: graphLabel(graphId),
        count: definition.callCount,
      }),
      confirmText: options.translate('workflow.graphs.delete_definition_cascade'),
      cancelText: options.translate('common.cancel'),
      color: 'error',
    })
    if (!accepted) return
    try {
      options.session.removeGraphCascade(graphId)
      options.clearSelection()
      void options.fitCurrentGraph()
    } catch (error) {
      reject(error)
    }
  }

  async function locateGraphCall(parentGraphId: string, callId: string): Promise<void> {
    const entry = options.session.source?.entryGraph
    if (!entry) return
    await options.focusNode(parentGraphId === entry ? [entry] : [entry, parentGraphId], callId)
  }

  function selectOnly(id: string): void {
    options.selectedNodeIds.value = new Set([id])
    options.selectedNodeId.value = id
  }

  function reject(error: unknown): void {
    options.showError(options.translate('workflow.toast.edit_rejected'), error)
  }

  return {
    graphDialogOpen,
    graphDialogMode,
    graphName,
    callableGraphIds,
    canInferGraphInterface,
    graphInterfaceCandidates,
    graphInterfaceReferenceCounts,
    selectedCall,
    selectedCallGraph,
    selectedCallPorts,
    graphLabel,
    openCalledGraph,
    openGraphAt,
    openGraphDialog,
    commitGraphDialog,
    inferGraphInterface,
    addGraphInterfaceCandidate,
    renameGraphInterfaceItem,
    moveGraphInterfaceItem,
    removeGraphInterfaceItem,
    addGraphCall,
    duplicateSelectedGraphCall,
    forkSelectedGraphCall,
    expandSelectedGraphCall,
    deleteGraphDefinition,
    duplicateGraphDefinition,
    deleteGraphDefinitionCascade,
    locateGraphCall,
  }
}
