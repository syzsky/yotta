import { computed, type Ref } from 'vue'
import type { Edge as FlowEdge, EdgeMouseEvent } from '@vue-flow/core'
import type { Edge, EditorSession } from './EditorSession'
import { graphHandle } from './graphHandles'
import { projectedTargetHandleChannel } from './connectionCompatibility'
import { graphBoundaryKeyFromEdge, type GraphBoundaryProjection } from './workflowGraphBoundary'
import { workflowEdgeVisualState } from './workflowEdgeVisualState'
import type { NodeRunStatus } from './runTrace'

interface WorkflowEdgeInteractionOptions {
  session: EditorSession
  graphBoundaryProjection: Readonly<Ref<GraphBoundaryProjection>>
  nodeRunStatusById: Readonly<Ref<ReadonlyMap<string, NodeRunStatus>>>
  selectedEdgeIds: Ref<Set<string>>
  applyCommand: (command: Parameters<EditorSession['apply']>[0]) => boolean
  selectEdge: (edgeId: string, additive: boolean) => void
  showError: (title: string, error: unknown) => void
  translate: (key: string) => string
}

export function useWorkflowEdgeInteractions(options: WorkflowEdgeInteractionOptions) {
  const flowEdges = computed<FlowEdge[]>(() => [
    ...(options.session.currentGraph?.edges ?? []).map((edge) => {
      const visual = workflowEdgeVisualState(edge, options.nodeRunStatusById.value)
      return {
        id: edgeId(edge),
        source: edge.from.nodeId,
        target: edge.to.nodeId,
        sourceHandle: graphHandle(edge.channel, 'output', edge.from.portId),
        targetHandle: targetHandle(edge),
        selected: options.selectedEdgeIds.value.has(edgeId(edge)),
        type: edge.presentation?.reroutes?.length ? 'reroute' : undefined,
        data: { edge },
        animated: visual.animated,
        style: { stroke: visual.stroke, strokeWidth: visual.strokeWidth },
      }
    }),
    ...options.graphBoundaryProjection.value.edges.map((edge) => ({
      ...edge,
      selected: options.selectedEdgeIds.value.has(edge.id),
    })),
  ])

  function targetHandle(edge: Edge): string {
    const node = options.session.currentGraph?.nodes.find(
      (candidate) => candidate.id === edge.to.nodeId,
    )
    const projection = node ? options.session.nodeInstanceProjection(node) : undefined
    const channel = projection
      ? projectedTargetHandleChannel(projection, edge.channel, edge.to.portId)
      : edge.channel
    return graphHandle(channel, 'input', edge.to.portId)
  }

  function disconnectEvent(event: EdgeMouseEvent): void {
    disconnect(event.edge.id)
  }

  function setReroutes(edge: Edge, reroutes: Array<{ x: number; y: number }>): void {
    options.applyCommand({ kind: 'set-edge-reroutes', edge, reroutes })
  }

  function selectedSourceEdge(): Edge | undefined {
    return options.selectedEdgeIds.value.size === 1 ? selectedSourceEdges()[0] : undefined
  }

  function selectedSourceEdges(): Edge[] {
    return (options.session.currentGraph?.edges ?? []).filter((edge) =>
      options.selectedEdgeIds.value.has(edgeId(edge)),
    )
  }

  function addReroute(): void {
    const edge = selectedSourceEdge()
    const graph = options.session.currentGraph
    if (!edge || !graph) return
    const position = (id: string) =>
      graph.nodes.find((node) => node.id === id)?.position ??
      graph.calls?.find((call) => call.id === id)?.position
    const from = position(edge.from.nodeId)
    const to = position(edge.to.nodeId)
    if (!from || !to) return
    const reroutes = [...(edge.presentation?.reroutes ?? [])]
    reroutes.push({ x: (from.x + to.x) / 2 + 115, y: (from.y + to.y) / 2 + 45 })
    setReroutes(edge, reroutes)
  }

  function clearReroutes(): void {
    const edge = selectedSourceEdge()
    if (edge) setReroutes(edge, [])
  }

  function disconnect(id: string): void {
    const projected = options.graphBoundaryProjection.value.edges.find((edge) => edge.id === id)
    const boundary = projected ? graphBoundaryKeyFromEdge(projected) : null
    if (boundary) {
      try {
        options.session.unbindGraphBoundary(boundary)
        options.selectedEdgeIds.value = new Set()
      } catch (error) {
        options.showError(options.translate('workflow.toast.edit_rejected'), error)
      }
      return
    }
    const edge = options.session.currentGraph?.edges.find((candidate) => edgeId(candidate) === id)
    if (edge && options.applyCommand({ kind: 'disconnect', edge }))
      options.selectedEdgeIds.value = new Set()
  }

  function select(event: EdgeMouseEvent): void {
    const source = event.event as MouseEvent | undefined
    options.selectEdge(event.edge.id, Boolean(source?.ctrlKey || source?.metaKey))
  }

  return {
    flowEdges,
    disconnectEvent,
    setReroutes,
    selectedSourceEdge,
    selectedSourceEdges,
    addReroute,
    clearReroutes,
    disconnect,
    select,
  }
}

export function workflowEdgeId(edge: {
  channel: string
  from: { nodeId: string; portId: string }
  to: { nodeId: string; portId: string }
}): string {
  return `${edge.channel}:${edge.from.nodeId}:${edge.from.portId}:${edge.to.nodeId}:${edge.to.portId}`
}

const edgeId = workflowEdgeId
