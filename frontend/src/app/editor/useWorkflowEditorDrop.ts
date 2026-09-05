import { ref } from 'vue'
import type { useAssetsStore } from '@/stores/assets'
import { snapshotGlobalAssetByID } from './workflowResourceSnapshot'
import { parseWorkspaceResource, RESOURCE_DRAG_FORMAT } from './resourceDrag'
import type { StateReferenceMode } from './EditorSession'

export const NODE_TYPE_DRAG_FORMAT = 'application/x-yotta-node-type'
export const STATE_REFERENCE_DRAG_FORMAT = 'application/x-yotta-state-reference'
export const SNIPPET_DRAG_FORMAT = 'application/x-yotta-snippet'
export const GRAPH_CALL_DRAG_FORMAT = 'application/x-yotta-graph-call'

interface WorkflowEditorDropOptions {
  assets: ReturnType<typeof useAssetsStore>
  screenToFlowCoordinate: (position: { x: number; y: number }) => { x: number; y: number }
  addNode: (nodeTypeId: string, position: { x: number; y: number }) => void
  useSnippet: (snippetId: string, position: { x: number; y: number }) => Promise<void>
  addGraphCall: (graphId: string, position: { x: number; y: number }) => void
  importWorkflowResource: (
    resource: Awaited<ReturnType<typeof snapshotGlobalAssetByID>>,
    position: { x: number; y: number },
  ) => void
  insertStateReference: (
    name: string,
    mode: StateReferenceMode,
    position: { x: number; y: number },
  ) => void
  translate: (key: string) => string
  showError: (title: string, error: unknown) => void
}

export function useWorkflowEditorDrop(options: WorkflowEditorDropOptions) {
  const active = ref(false)
  const formats = [
    NODE_TYPE_DRAG_FORMAT,
    STATE_REFERENCE_DRAG_FORMAT,
    SNIPPET_DRAG_FORMAT,
    GRAPH_CALL_DRAG_FORMAT,
    RESOURCE_DRAG_FORMAT,
  ]

  function continueDrag(event: DragEvent): void {
    if (!formats.some((format) => event.dataTransfer?.types.includes(format))) return
    event.preventDefault()
    if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
    active.value = true
  }

  function finishDrag(): void {
    active.value = false
  }

  function drop(event: DragEvent): void {
    const nodeTypeId = event.dataTransfer?.getData(NODE_TYPE_DRAG_FORMAT)
    const stateReference = event.dataTransfer?.getData(STATE_REFERENCE_DRAG_FORMAT)
    const snippetID = event.dataTransfer?.getData(SNIPPET_DRAG_FORMAT)
    const graphCallID = event.dataTransfer?.getData(GRAPH_CALL_DRAG_FORMAT)
    const workspaceResource = event.dataTransfer?.getData(RESOURCE_DRAG_FORMAT)
    if (nodeTypeId || stateReference || snippetID || graphCallID || workspaceResource)
      event.preventDefault()
    finishDrag()
    const position = options.screenToFlowCoordinate({ x: event.clientX, y: event.clientY })
    if (nodeTypeId) return options.addNode(nodeTypeId, position)
    if (snippetID) return void options.useSnippet(snippetID, position)
    if (graphCallID) return options.addGraphCall(graphCallID, position)
    if (workspaceResource) return void dropWorkspaceResource(workspaceResource, position)
    const parsed = parseStateReference(stateReference)
    if (parsed) options.insertStateReference(parsed.name, parsed.mode, position)
  }

  async function dropWorkspaceResource(
    raw: string,
    position: { x: number; y: number },
  ): Promise<void> {
    const guid = parseWorkspaceResource(raw)
    if (!guid) return
    try {
      options.importWorkflowResource(await snapshotGlobalAssetByID(guid), position)
      options.assets.markUsed(guid)
    } catch (error) {
      options.showError(options.translate('workflow.toast.edit_rejected'), error)
    }
  }

  function parseStateReference(raw?: string): { name: string; mode: 'read' | 'write' } | null {
    if (!raw) return null
    try {
      const value = JSON.parse(raw) as Record<string, unknown>
      return typeof value.name === 'string' && (value.mode === 'read' || value.mode === 'write')
        ? { name: value.name, mode: value.mode }
        : null
    } catch {
      return null
    }
  }

  return { active, continueDrag, finishDrag, drop }
}
