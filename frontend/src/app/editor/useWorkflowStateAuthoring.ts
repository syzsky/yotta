import { computed, type Ref } from 'vue'
import type { TypeExpression } from '../../../../contracts/workflow/current/workflow-source'
import type { EditorSession, StateReferenceMode } from './EditorSession'
import type { PendingStatePromotion } from './useWorkflowConnectionAuthoring'

interface WorkflowStateAuthoringOptions {
  session: EditorSession
  canvasElement: Ref<HTMLElement | null>
  pendingPromotion: Ref<PendingStatePromotion | null>
  promotionName: Ref<string>
  selectedNodeId: Ref<string>
  selectedNodeIds: Ref<Set<string>>
  screenToFlowCoordinate: (position: { x: number; y: number }) => { x: number; y: number }
  focusNode: (graphPath: string[], nodeId: string) => Promise<void>
  showVariables: () => void
  translate: (key: string) => string
  showError: (title: string, error: unknown) => void
}

export function useWorkflowStateAuthoring(options: WorkflowStateAuthoringOptions) {
  const referenceLocations = computed<
    Record<string, Array<{ graphId: string; nodeId: string; mode: 'read' | 'write' }>>
  >(() => {
    const result: Record<
      string,
      Array<{ graphId: string; nodeId: string; mode: 'read' | 'write' }>
    > = {}
    for (const graph of options.session.source?.graphs ?? []) {
      for (const node of graph.nodes) {
        if (!node.nodeRef.nodeTypeId.includes('/nodes/state/')) continue
        const variable = node.config.variable
        if (typeof variable !== 'string') continue
        ;(result[variable] ??= []).push({
          graphId: graph.id,
          nodeId: node.id,
          mode: node.nodeRef.nodeTypeId.endsWith('/write') ? 'write' : 'read',
        })
      }
    }
    return result
  })

  const promotionError = computed(() => {
    const name = options.promotionName.value.trim()
    if (!/^[A-Za-z0-9_][A-Za-z0-9._-]*$/.test(name))
      return options.translate('workflow.state_panel.promote_invalid_name')
    if (options.session.source?.variables.some((variable) => variable.name === name))
      return options.translate('workflow.state_panel.promote_duplicate_name')
    return ''
  })

  function insertAtCenter(name: string, mode: StateReferenceMode): void {
    const rect = options.canvasElement.value?.getBoundingClientRect()
    insert(
      name,
      mode,
      rect
        ? options.screenToFlowCoordinate({
            x: rect.left + rect.width / 2,
            y: rect.top + rect.height / 2,
          })
        : { x: 160, y: 160 },
    )
  }

  function insert(
    name: string,
    mode: StateReferenceMode,
    position: { x: number; y: number },
  ): void {
    try {
      selectOnly(options.session.insertStateReference(name, mode, position))
    } catch (error) {
      options.showError(options.translate('workflow.state_panel.insert_failed'), error)
    }
  }

  async function locate(name: string): Promise<void> {
    const reference = referenceLocations.value[name]?.[0]
    if (reference) await locateAt(reference.graphId, reference.nodeId)
  }

  async function locateAt(graphId: string, nodeId: string): Promise<void> {
    const source = options.session.source
    if (!source?.graphs.some((graph) => graph.id === graphId)) return
    await options.focusNode([source.entryGraph, graphId], nodeId)
  }

  function typeChangeImpact(name: string, type: TypeExpression) {
    return options.session.stateTypeChangeImpact(name, structuredClone(type))
  }

  function uniqueName(base: string): string {
    const normalized = base.replace(/[^A-Za-z0-9._-]+/g, '_').replace(/^[^A-Za-z0-9_]+/, '')
    const prefix = normalized || 'value'
    const existing = new Set(
      (options.session.source?.variables ?? []).map((variable) => variable.name),
    )
    if (!existing.has(prefix)) return prefix
    for (let index = 2; index <= 4096; index++) {
      const candidate = `${prefix}_${index}`
      if (!existing.has(candidate)) return candidate
    }
    return `${prefix}_${Date.now()}`
  }

  function commitPromotion(): void {
    const pending = options.pendingPromotion.value
    if (!pending || promotionError.value) return
    try {
      selectOnly(
        options.session.promoteOutputToState(
          pending.nodeId,
          pending.portId,
          options.promotionName.value.trim(),
          pending.position,
        ),
      )
      options.showVariables()
      cancelPromotion()
    } catch (error) {
      options.showError(options.translate('workflow.state_panel.promote_failed'), error)
    }
  }

  function cancelPromotion(): void {
    options.pendingPromotion.value = null
    options.promotionName.value = ''
  }

  function selectOnly(nodeId: string): void {
    options.selectedNodeIds.value = new Set([nodeId])
    options.selectedNodeId.value = nodeId
  }

  return {
    referenceLocations,
    promotionError,
    insertAtCenter,
    insert,
    locate,
    locateAt,
    typeChangeImpact,
    uniqueName,
    commitPromotion,
    cancelPromotion,
  }
}
