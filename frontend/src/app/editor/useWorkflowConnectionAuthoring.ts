import type { Connection, OnConnectStartParams } from '@vue-flow/core'
import { ref, type Ref } from 'vue'
import type { Edge } from '../../../../contracts/workflow/current/workflow-source'
import type { EditorCommand, EditorSession } from './EditorSession'
import { parseGraphHandle, type ParsedHandle } from './graphHandles'
import type { ConversionCandidatePlan, ConnectionIssue } from './connectionCompatibility'
import { graphBoundaryBindingFromConnection, isGraphBoundaryNodeId } from './workflowGraphBoundary'
import type { WorkflowConnectionCandidate } from './WorkflowConnectionMenu.vue'

export interface ConnectionAnchor {
  nodeId: string
  handle: ParsedHandle
}

export interface ConnectionMenuState {
  anchor: ConnectionAnchor
  flowPosition: { x: number; y: number }
  canvasPosition: { x: number; y: number }
}

export interface PendingConversion {
  edge: Edge
  candidates: ConversionCandidatePlan[]
  sourceType: string
  targetType: string
  position: { x: number; y: number }
}

export interface PendingStatePromotion {
  nodeId: string
  portId: string
  position: { x: number; y: number }
  typeLabel: string
}

interface ConnectionAuthoringOptions {
  session: EditorSession
  canvasElement: Ref<HTMLElement | null>
  selectedNodeId: Ref<string>
  selectedNodeIds: Ref<Set<string>>
  applyCommand: (command: EditorCommand) => boolean
  addNode: (nodeTypeId: string, position: { x: number; y: number }) => void
  screenToFlowCoordinate: (point: { x: number; y: number }) => { x: number; y: number }
  uniqueStateName: (base: string) => string
  translate: (key: string, params?: Record<string, unknown>) => string
  translationExists: (key: string) => boolean
  showError: (title: string, error: unknown) => void
}

export function useWorkflowConnectionAuthoring(options: ConnectionAuthoringOptions) {
  const connectionStart = ref<ConnectionAnchor | null>(null)
  const connectionMenu = ref<ConnectionMenuState | null>(null)
  const pendingConversion = ref<PendingConversion | null>(null)
  const pendingStatePromotion = ref<PendingStatePromotion | null>(null)
  const statePromotionName = ref('')
  const connectionHint = ref('')
  const connectionError = ref('')
  let connectionEndTimer: ReturnType<typeof setTimeout> | undefined
  let connectionMadeThisGesture = false

  function connect(connection: Connection): void {
    const graph = options.session.currentGraph
    const boundary = graph ? graphBoundaryBindingFromConnection(connection, graph) : null
    if (boundary) {
      const compatibility = options.session.graphBoundaryCompatibility(boundary)
      if (!compatibility.valid) {
        connectionHint.value = issueText(compatibility)
        return
      }
      try {
        options.session.bindGraphBoundary(boundary)
        connectionMadeThisGesture = true
        connectionHint.value = ''
      } catch (error) {
        options.showError(options.translate('workflow.toast.edit_rejected'), error)
      }
      return
    }
    const edge = connectionEdge(connection)
    if (!edge) return
    const compatibility = options.session.connectionCompatibility(edge)
    if (!compatibility.valid) {
      if (
        compatibility.conversions?.length &&
        compatibility.sourceType &&
        compatibility.targetType
      ) {
        pendingConversion.value = {
          edge,
          candidates: compatibility.conversions,
          sourceType: compatibility.sourceType,
          targetType: compatibility.targetType,
          position: conversionNodePosition(edge),
        }
        connectionMadeThisGesture = true
        connectionHint.value = ''
        return
      }
      connectionHint.value = issueText(compatibility)
      return
    }
    if (options.applyCommand({ kind: 'connect', edge })) {
      connectionMadeThisGesture = true
      connectionHint.value = ''
    }
  }

  function isValidConnection(connection: Connection): boolean {
    const graph = options.session.currentGraph
    const boundary = graph ? graphBoundaryBindingFromConnection(connection, graph) : null
    if (boundary) {
      const compatibility = options.session.graphBoundaryCompatibility(boundary)
      connectionHint.value = compatibility.valid ? '' : issueText(compatibility)
      return compatibility.valid
    }
    const edge = connectionEdge(connection)
    if (!edge) return false
    const compatibility = options.session.connectionCompatibility(edge)
    const conversionAvailable = Boolean(compatibility.conversions?.length)
    connectionHint.value =
      compatibility.valid || conversionAvailable ? '' : issueText(compatibility)
    return compatibility.valid || conversionAvailable
  }

  function startConnection(params: OnConnectStartParams): void {
    connectionMadeThisGesture = false
    connectionHint.value = ''
    closeConnectionMenu()
    const handle = parseGraphHandle(params.handleId)
    connectionStart.value = params.nodeId && handle ? { nodeId: params.nodeId, handle } : null
  }

  function endConnection(event?: MouseEvent | TouchEvent): void {
    const anchor = connectionStart.value
    connectionStart.value = null
    const point = eventClientPoint(event)
    clearTimeout(connectionEndTimer)
    connectionEndTimer = setTimeout(() => {
      if (connectionMadeThisGesture) {
        connectionMadeThisGesture = false
        return
      }
      if (anchor && point && !isGraphBoundaryNodeId(anchor.nodeId))
        openConnectionMenu(anchor, point)
    }, 0)
  }

  function closeConnectionMenu(): void {
    connectionMenu.value = null
    connectionError.value = ''
  }

  function selectConnectionCandidate(candidate: WorkflowConnectionCandidate): void {
    const menu = connectionMenu.value
    if (!menu) return
    connectionError.value = ''
    const position = { ...menu.flowPosition }
    if (candidate.promoteState) {
      const anchorNode = options.session.currentGraph?.nodes.find(
        (node) => node.id === menu.anchor.nodeId,
      )
      const projection = anchorNode ? options.session.nodeInstanceProjection(anchorNode) : undefined
      const output = projection?.dataOutputs.find(
        (port) => menu.anchor.handle.channel === 'data' && port.id === menu.anchor.handle.portId,
      )
      if (!output) return
      pendingStatePromotion.value = {
        nodeId: menu.anchor.nodeId,
        portId: menu.anchor.handle.portId,
        position,
        typeLabel: output.type.label,
      }
      statePromotionName.value = options.uniqueStateName(menu.anchor.handle.portId)
      closeConnectionMenu()
      return
    }
    if (!candidate.handle) {
      options.addNode(candidate.nodeTypeId, position)
      closeConnectionMenu()
      return
    }
    try {
      const nodeId = options.session.insertConnectedNode(
        menu.anchor.nodeId,
        menu.anchor.handle,
        candidate.nodeTypeId,
        candidate.handle,
        position,
      )
      options.selectedNodeId.value = nodeId
      options.selectedNodeIds.value = new Set([nodeId])
      closeConnectionMenu()
    } catch {
      connectionError.value = options.translate('error.workflow.connection.invalid')
      options.showError(options.translate('workflow.toast.edit_rejected'), {
        id: 'workflow.connection.invalid',
      })
    }
  }

  function applyConversion(candidate: ConversionCandidatePlan): void {
    const pending = pendingConversion.value
    if (!pending) return
    try {
      const nodeId = options.session.insertConversionBridge(
        pending.edge,
        candidate,
        pending.position,
      )
      options.selectedNodeIds.value = new Set([nodeId])
      options.selectedNodeId.value = nodeId
      pendingConversion.value = null
      connectionMadeThisGesture = true
    } catch (error) {
      options.showError(options.translate('workflow.connection.conversion_failed'), error)
    }
  }

  function cancelConversion(): void {
    pendingConversion.value = null
  }

  function conversionTitle(candidate: ConversionCandidatePlan): string {
    if (candidate.titleKey && options.translationExists(candidate.titleKey)) {
      return options.translate(candidate.titleKey)
    }
    return candidate.nodeTypeId.split('/').at(-1) ?? candidate.nodeTypeId
  }

  function conversionNodePosition(edge: Edge): { x: number; y: number } {
    const graph = options.session.currentGraph
    const position = (nodeId: string) =>
      graph?.nodes.find((node) => node.id === nodeId)?.position ??
      graph?.calls?.find((call) => call.id === nodeId)?.position
    const source = position(edge.from.nodeId)
    const target = position(edge.to.nodeId)
    if (!source || !target) return { x: 160, y: 160 }
    return { x: (source.x + target.x) / 2, y: (source.y + target.y) / 2 }
  }

  function dispose(): void {
    clearTimeout(connectionEndTimer)
    connectionEndTimer = undefined
  }

  function openConnectionMenu(anchor: ConnectionAnchor, point: { x: number; y: number }): void {
    const bounds = options.canvasElement.value?.getBoundingClientRect()
    if (!bounds) return
    connectionMenu.value = {
      anchor,
      flowPosition: options.screenToFlowCoordinate(point),
      canvasPosition: {
        x: Math.max(8, Math.min(point.x - bounds.left, Math.max(8, bounds.width - 328))),
        y: Math.max(8, Math.min(point.y - bounds.top, Math.max(8, bounds.height - 424))),
      },
    }
    connectionHint.value = ''
    connectionError.value = ''
  }

  function issueText(compatibility: {
    issue?: ConnectionIssue
    sourceType?: string
    targetType?: string
  }): string {
    if (compatibility.issue === 'type' && compatibility.sourceType && compatibility.targetType) {
      return options.translate('workflow.connection.issue.type_detail', {
        source: compatibility.sourceType,
        target: compatibility.targetType,
      })
    }
    return options.translate(`workflow.connection.issue.${compatibility.issue ?? 'port'}`)
  }

  return {
    connectionMenu,
    pendingConversion,
    pendingStatePromotion,
    statePromotionName,
    connectionHint,
    connectionError,
    connect,
    isValidConnection,
    startConnection,
    endConnection,
    closeConnectionMenu,
    selectConnectionCandidate,
    applyConversion,
    cancelConversion,
    conversionTitle,
    conversionNodePosition,
    dispose,
  }
}

function connectionEdge(connection: Connection): Edge | null {
  const source = parseGraphHandle(connection.sourceHandle)
  const target = parseGraphHandle(connection.targetHandle)
  if (!source || !target || source.direction !== 'output' || target.direction !== 'input')
    return null
  return {
    channel: source.channel,
    from: { nodeId: connection.source, portId: source.portId },
    to: { nodeId: connection.target, portId: target.portId },
  }
}

function eventClientPoint(event?: MouseEvent | TouchEvent): { x: number; y: number } | null {
  if (!event) return null
  if (event instanceof MouseEvent) return { x: event.clientX, y: event.clientY }
  const touch = event.changedTouches[0]
  return touch ? { x: touch.clientX, y: touch.clientY } : null
}
