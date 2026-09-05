import { nextTick, ref, type Ref } from 'vue'
import type { GraphNode, NodeChange, NodeDragEvent, NodeMouseEvent } from '@vue-flow/core'
import {
  canvasOwnsWheelTarget,
  mergeMarqueeSelection,
  zoomViewportAtPoint,
} from './workflowCanvasInteraction'

interface WorkflowCanvasGestureOptions {
  canvasElement: Ref<HTMLElement | null>
  selectedNodeId: Ref<string>
  selectedNodeIds: Ref<Set<string>>
  selectedEdgeIds: Ref<Set<string>>
  inspectorAutoOpen: Readonly<Ref<boolean>>
  getSelectedNodes: () => GraphNode[]
  findNode: (nodeId: string) => GraphNode | undefined
  addSelectedNodes: (nodes: GraphNode[]) => void
  removeSelectedNodes: (nodes: GraphNode[]) => void
  getViewport: () => { x: number; y: number; zoom: number }
  setViewport: (viewport: { x: number; y: number; zoom: number }) => Promise<unknown> | unknown
  closeConnectionMenu: () => void
  executeSelectionClear: () => void
  showNodeInspector: () => void
  dragPositions: (
    event: NodeDragEvent,
  ) => Array<{ nodeId: string; position: { x: number; y: number } }>
  trackLivePosition: (nodeId: string, position: { x: number; y: number }) => void
  updateLivePosition: (nodeId: string, position: { x: number; y: number }) => void
  applyPositions: (positions: Array<{ nodeId: string; position: { x: number; y: number } }>) => void
  clearLivePosition: (nodeId: string) => void
  clearGuides: () => void
}

export function useWorkflowCanvasGestures(options: WorkflowCanvasGestureOptions) {
  const pointerInside = ref(false)
  const lastPointer = ref<{ x: number; y: number } | null>(null)
  let marqueeBase = new Set<string>()
  let marqueeActive = false

  function enter(): void {
    pointerInside.value = true
  }

  function leave(): void {
    pointerInside.value = false
  }

  function trackPointer(event: PointerEvent): void {
    lastPointer.value = { x: event.clientX, y: event.clientY }
  }

  function paneClick(): void {
    clearSelection()
    options.closeConnectionMenu()
  }

  function captureMarquee(event: PointerEvent): void {
    const target = event.target as HTMLElement | null
    marqueeActive =
      event.button === 0 && event.shiftKey && target?.classList.contains('vue-flow__pane') === true
    marqueeBase = marqueeActive ? new Set(options.selectedNodeIds.value) : new Set()
  }

  async function finishMarquee(): Promise<void> {
    if (!marqueeActive) return
    marqueeActive = false
    await nextTick()
    const merged = mergeMarqueeSelection(marqueeBase, options.selectedNodeIds.value)
    marqueeBase = new Set()
    const nodes = [...merged].flatMap((nodeId) => {
      const node = options.findNode(nodeId)
      return node ? [node] : []
    })
    if (nodes.length) options.addSelectedNodes(nodes)
    options.selectedNodeIds.value = merged
    options.selectedNodeId.value = [...merged].at(-1) ?? ''
    options.selectedEdgeIds.value = new Set()
  }

  function wheel(event: WheelEvent): void {
    const canvas = options.canvasElement.value
    if (!canvas || !canvasOwnsWheelTarget(event.target)) return
    const rect = canvas.getBoundingClientRect()
    const viewport = options.getViewport()
    const next = zoomViewportAtPoint(
      viewport,
      { x: event.clientX - rect.left, y: event.clientY - rect.top },
      event.deltaY,
      event.deltaMode,
    )
    if (next.zoom === viewport.zoom) return
    event.preventDefault()
    event.stopPropagation()
    void options.setViewport(next)
  }

  function clearSelection(): void {
    marqueeActive = false
    marqueeBase = new Set()
    options.executeSelectionClear()
  }

  function selectNode(event: NodeMouseEvent): void {
    options.selectedEdgeIds.value = new Set()
    options.selectedNodeId.value = event.node.id
    options.showNodeInspector()
    const source = event.event as MouseEvent | undefined
    if (!source?.shiftKey && !source?.ctrlKey && !source?.metaKey) {
      options.selectedNodeIds.value = new Set([event.node.id])
    }
  }

  function selectNodeForContextMenu(nodeId: string): void {
    options.selectedEdgeIds.value = new Set()
    options.selectedNodeId.value = nodeId
    options.showNodeInspector()
    if (options.selectedNodeIds.value.has(nodeId)) return
    options.removeSelectedNodes(options.getSelectedNodes())
    const node = options.findNode(nodeId)
    if (node) options.addSelectedNodes([node])
    options.selectedNodeIds.value = new Set([nodeId])
  }

  function nodesChanged(changes: NodeChange[]): void {
    const selected = new Set(options.selectedNodeIds.value)
    let changed = false
    for (const change of changes) {
      if (change.type !== 'select') continue
      changed = true
      if (change.selected) selected.add(change.id)
      else selected.delete(change.id)
    }
    if (!changed) return
    options.selectedNodeIds.value = selected
    if (!selected.has(options.selectedNodeId.value))
      options.selectedNodeId.value = [...selected].at(-1) ?? ''
    if (selected.size) options.selectedEdgeIds.value = new Set()
  }

  function trackNodeDrag(event: NodeDragEvent): void {
    const positions = options.dragPositions(event)
    for (const item of positions) {
      options.trackLivePosition(item.nodeId, item.position)
      options.updateLivePosition(item.nodeId, item.position)
    }
  }

  function finishNodeDrag(event: NodeDragEvent): void {
    const positions = options.dragPositions(event)
    for (const item of positions) options.trackLivePosition(item.nodeId, item.position)
    options.applyPositions(positions)
    for (const item of positions) options.clearLivePosition(item.nodeId)
    options.clearGuides()
  }

  return {
    pointerInside,
    lastPointer,
    enter,
    leave,
    trackPointer,
    paneClick,
    captureMarquee,
    finishMarquee,
    wheel,
    clearSelection,
    selectNode,
    selectNodeForContextMenu,
    nodesChanged,
    trackNodeDrag,
    finishNodeDrag,
  }
}
