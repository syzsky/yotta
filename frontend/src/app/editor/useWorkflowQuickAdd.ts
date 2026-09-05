import { computed, ref, type Ref } from 'vue'
import type { useSnippetsStore } from '@/stores/snippets'
import type { Edge, EditorSession, NodeProjection } from './EditorSession'
import type { WorkflowQuickAddItem } from './workflowQuickAdd'

interface WorkflowQuickAddOptions {
  session: EditorSession
  snippets: ReturnType<typeof useSnippetsStore>
  canvasElement: Ref<HTMLElement | null>
  catalogNodes: Readonly<Ref<NodeProjection[]>>
  selectedEdgeIds: Ref<Set<string>>
  selectedNodeId: Ref<string>
  selectedNodeIds: Ref<Set<string>>
  lastCanvasPointer: Ref<{ x: number; y: number } | null>
  selectedSourceEdges: () => Edge[]
  conversionNodePosition: (edge: Edge) => { x: number; y: number }
  screenToFlowCoordinate: (position: { x: number; y: number }) => { x: number; y: number }
  viewport: () => { x: number; y: number; zoom: number }
  canvasAssistCollapsed: () => boolean
  addNode: (nodeTypeId: string, position: { x: number; y: number }) => void
  useSnippet: (snippetId: string, position: { x: number; y: number }) => Promise<void>
  projectionTitle: (projection: NodeProjection) => string
  categoryLabel: (category: string) => string
  catalogSearchText: (projection: NodeProjection) => string
  translate: (key: string) => string
  translationExists: (key: string) => boolean
  showError: (title: string, error: unknown) => void
}

export function useWorkflowQuickAdd(options: WorkflowQuickAddOptions) {
  const open = ref(false)
  const position = ref({ x: 160, y: 160 })
  const anchor = ref({ x: 160, y: 160 })
  const intent = ref<'add' | 'insert-edge'>('add')

  const items = computed<WorkflowQuickAddItem[]>(() => [
    ...options.catalogNodes.value.map((projection) => {
      const description =
        projection.descriptionKey && options.translationExists(projection.descriptionKey)
          ? options.translate(projection.descriptionKey)
          : projection.nodeRef.nodeTypeId
      const category = `node:${projection.category || 'other'}`
      return {
        id: projection.nodeRef.nodeTypeId,
        kind: 'node' as const,
        title: options.projectionTitle(projection),
        description,
        category,
        categoryLabel: options.categoryLabel(projection.category || 'other'),
        icon: `i-tabler-${projection.icon || 'box'}`,
        searchText: options.catalogSearchText(projection),
      }
    }),
    ...options.snippets.items.map((snippet) => ({
      id: snippet.id,
      kind: 'snippet' as const,
      title: snippet.name,
      description: snippet.description || snippet.nodeTypeId,
      category: 'snippet:all',
      categoryLabel: options.translate('workflow.snippets.title'),
      icon: 'i-tabler-bookmark',
      shortcut: snippet.shortcut,
      searchText: [
        snippet.name,
        snippet.description,
        snippet.category,
        snippet.tags.join(' '),
        snippet.nodeTypeId,
        snippet.shortcut,
      ]
        .filter(Boolean)
        .join(' ')
        .toLocaleLowerCase(),
    })),
  ])

  const insertableItems = computed(() => {
    const edges = options.selectedSourceEdges()
    if (!edges.length || edges.length !== options.selectedEdgeIds.value.size) return []
    return items.value.filter((item) => {
      if (item.kind !== 'node') return false
      const projection = options.session.nodeProjection(item.id)
      return edges.every(
        (edge) =>
          edge.channel !== 'data' &&
          projection?.signals.some(
            (signal) => signal.direction === 'input' && signal.channel === edge.channel,
          ) &&
          projection.signals.some(
            (signal) => signal.direction === 'output' && signal.channel === edge.channel,
          ),
      )
    })
  })

  function trackPointer(event: PointerEvent): void {
    options.lastCanvasPointer.value = { x: event.clientX, y: event.clientY }
  }

  function insertionPosition(): { x: number; y: number } {
    const rect = options.canvasElement.value?.getBoundingClientRect()
    const point =
      options.lastCanvasPointer.value ??
      ({
        x: (rect?.left ?? 0) + (rect?.width ?? 320) / 2,
        y: (rect?.top ?? 0) + (rect?.height ?? 320) / 2,
      } as const)
    const viewport = options.viewport()
    const zoom = Number.isFinite(viewport.zoom) && viewport.zoom > 0 ? viewport.zoom : 1
    const x = (point.x - (rect?.left ?? 0) - viewport.x) / zoom
    const y = (point.y - (rect?.top ?? 0) - viewport.y) / zoom
    return { x: Number.isFinite(x) ? x : 160, y: Number.isFinite(y) ? y : 160 }
  }

  function show(): void {
    intent.value = 'add'
    position.value = insertionPosition()
    anchor.value = pointerOrCenter()
    open.value = true
  }

  function showFromAssist(): void {
    intent.value = 'add'
    const rect = options.canvasElement.value?.getBoundingClientRect()
    position.value = rect
      ? options.screenToFlowCoordinate({
          x: rect.left + rect.width / 2,
          y: rect.top + rect.height / 2,
        })
      : { x: 160, y: 160 }
    anchor.value = assistAnchor()
    open.value = true
  }

  function showInsertFromAssist(): void {
    const edges = options.selectedSourceEdges()
    if (!edges.length || edges.length !== options.selectedEdgeIds.value.size) return
    intent.value = 'insert-edge'
    position.value = options.conversionNodePosition(edges[0]!)
    anchor.value = assistAnchor()
    open.value = true
  }

  function choose(item: WorkflowQuickAddItem): void {
    const insertion = { ...position.value }
    if (intent.value === 'insert-edge' && item.kind === 'node') {
      const edges = options.selectedSourceEdges()
      if (!edges.length || edges.length !== options.selectedEdgeIds.value.size) return
      try {
        const nodeIds = options.session.insertNodesIntoSignalEdges(
          edges,
          item.id,
          edges.map((edge) => options.conversionNodePosition(edge)),
        )
        options.selectedNodeIds.value = new Set(nodeIds)
        options.selectedNodeId.value = nodeIds.at(-1) ?? ''
        options.selectedEdgeIds.value = new Set()
      } catch (error) {
        options.showError(options.translate('workflow.canvas_assist.insert_failed'), error)
      }
      return
    }
    if (item.kind === 'node') options.addNode(item.id, insertion)
    else void options.useSnippet(item.id, insertion)
  }

  function pointerOrCenter(): { x: number; y: number } {
    if (options.lastCanvasPointer.value) return options.lastCanvasPointer.value
    const rect = options.canvasElement.value?.getBoundingClientRect()
    return {
      x: (rect?.left ?? 0) + (rect?.width ?? 320) / 2,
      y: (rect?.top ?? 0) + (rect?.height ?? 320) / 2,
    }
  }

  function assistAnchor(): { x: number; y: number } {
    const rect = options.canvasElement.value?.getBoundingClientRect()
    return {
      x: (rect?.left ?? 0) + (options.canvasAssistCollapsed() ? 56 : 176),
      y: (rect?.top ?? 0) + 64,
    }
  }

  return {
    open,
    position,
    anchor,
    intent,
    items,
    insertableItems,
    trackPointer,
    insertionPosition,
    show,
    showFromAssist,
    showInsertFromAssist,
    choose,
  }
}
