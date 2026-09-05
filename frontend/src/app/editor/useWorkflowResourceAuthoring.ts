import { ref, type Ref } from 'vue'
import type { AssetPickerSelection, useAssetsStore } from '@/stores/assets'
import type { WorkflowResource } from '../../../../contracts/workflow/current/workflow-source'
import { normalizeError } from '@/lib/invoke'
import type { EditorCommand, EditorSession, Node } from './EditorSession'
import type { ResourceLocateRequest, ResourceLocation } from './resourceLocator'
import { applyCapturedImageVersion, WorkflowResourceVersionError } from './workflowResourceVersions'

interface WorkflowResourceAuthoringOptions {
  session: EditorSession
  assets: ReturnType<typeof useAssetsStore>
  canvasElement: Ref<HTMLElement | null>
  selectedNode: Readonly<Ref<Node | null>>
  selectedNodeId: Ref<string>
  selectedNodeIds: Ref<Set<string>>
  defaultTargetSlot: Readonly<Ref<string>>
  recordingTargetSlot: () => string
  recordingTargetItems: Readonly<Ref<Array<{ value: string }>>>
  screenToFlowCoordinate: (position: { x: number; y: number }) => { x: number; y: number }
  applyCommand: (command: EditorCommand) => boolean
  selectNodeForContextMenu: (nodeId: string) => void
  showResourcePanel: (kind: ResourceLocation['kind']) => void
  openScreenPicker: (
    mode: 'workflow_resource' | 'workflow_resource_version',
    id: string,
    targetSlot: string,
  ) => Promise<void>
  waitForPickerResult: (id: string) => Promise<{
    id: string
    payload?: { cancelled?: boolean; resource?: WorkflowResource }
  }>
  translate: (key: string) => string
  showError: (title: string, error: unknown) => void
}

export function useWorkflowResourceAuthoring(options: WorkflowResourceAuthoringOptions) {
  const locateRequest = ref<ResourceLocateRequest | null>(null)
  let locateSequence = 0
  const captureOpen = ref(false)
  const captureTargetSlot = ref('')
  const captureBusy = ref(false)
  const captureIntent = ref<{
    mode: 'replace' | 'append'
    resource: WorkflowResource
    variantId?: string
  } | null>(null)

  function openCapture(
    resource?: WorkflowResource,
    mode?: 'replace' | 'append',
    variantId?: string,
  ): void {
    const targets = options.recordingTargetItems.value
    if (!targets.length) {
      options.showError(
        options.translate('assets.templates.capture_failed'),
        options.translate('workflow.inspector.no_installed_target'),
      )
      return
    }
    const selectedSlot = options.selectedNode.value?.config.slot
    captureTargetSlot.value =
      typeof selectedSlot === 'string' && targets.some((item) => item.value === selectedSlot)
        ? selectedSlot
        : options.defaultTargetSlot.value || captureTargetSlot.value || targets[0]?.value || ''
    captureIntent.value = resource && mode ? { resource: copy(resource), mode, variantId } : null
    captureOpen.value = true
  }

  function openRecapture(resource: WorkflowResource, variantId: string): void {
    openCapture(resource, 'replace', variantId)
  }

  function captureForNode(nodeId: string): void {
    options.selectNodeForContextMenu(nodeId)
    openCapture()
  }

  async function captureWorkspaceTemplate(): Promise<void> {
    if (!captureTargetSlot.value) return
    captureBusy.value = true
    const id = `workflow-template-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    try {
      const resultPromise = options.waitForPickerResult(id)
      await options.openScreenPicker(
        captureIntent.value ? 'workflow_resource_version' : 'workflow_resource',
        id,
        captureTargetSlot.value,
      )
      captureOpen.value = false
      const result = await resultPromise
      const resource = result.payload?.resource
      if (!resource || result.payload?.cancelled) return
      const intent = captureIntent.value
      if (!intent) {
        importResource(resource)
        return
      }
      const captured = resource.image?.variants[0]
      if (!captured)
        throw new WorkflowResourceVersionError('workflow.resource.capture_result_invalid')
      options.session.apply({
        kind: 'replace-resource',
        resourceId: intent.resource.id,
        resource: applyCapturedImageVersion(
          intent.resource,
          captured,
          intent.mode,
          intent.variantId,
        ),
      })
    } catch (error) {
      const normalized = normalizeError(error)
      const feedback =
        error instanceof WorkflowResourceVersionError
          ? { id: error.id }
          : normalized.id || normalized.errors?.length
            ? error
            : { id: 'workflow.resource.capture_apply_failed' }
      options.showError(options.translate('assets.templates.capture_failed'), feedback)
    } finally {
      captureBusy.value = false
      captureIntent.value = null
    }
  }

  function removeVariant(resource: WorkflowResource, variantId: string): void {
    const variants = resource.image?.variants ?? []
    if (variants.length <= 1 || !variantId) return
    const remaining = variants.filter((variant) => variant.id !== variantId)
    const first = remaining[0]
    if (!first || remaining.length === variants.length) return
    try {
      options.session.apply({
        kind: 'replace-resource',
        resourceId: resource.id,
        resource: {
          ...copy(resource),
          image: { variants: [first, ...remaining.slice(1)] },
        },
      })
    } catch (error) {
      reject(error)
    }
  }

  function useWorkspaceResource(
    selection: AssetPickerSelection,
    dropPosition?: { x: number; y: number },
  ): void {
    const portId =
      selection.kind === 'macro' ? 'macro' : selection.kind === 'clip' ? 'clip' : 'template'
    const current = options.selectedNode.value
    const projection = current
      ? options.session.nodeProjection(current.nodeRef.nodeTypeId)
      : undefined
    if (
      current &&
      projection?.dataInputs.some(
        (port) =>
          port.id === portId &&
          port.type.representations.some((representation) => representation.kind === 'blob-ref'),
      ) &&
      options.applyCommand({
        kind: 'bind-blob',
        nodeId: current.id,
        portId,
        blob: { ...selection.blob },
      })
    ) {
      options.assets.markUsed(selection.guid)
      return
    }

    const nodeTypeId =
      selection.kind === 'macro'
        ? 'https://schemas.yotta.dev/nodes/automation/play-macro'
        : selection.kind === 'clip'
          ? 'https://schemas.yotta.dev/nodes/automation/play-input-clip'
          : 'https://schemas.yotta.dev/nodes/automation/click-template'
    try {
      const ids = options.session.insertLinearDraft(
        [
          {
            nodeTypeID: nodeTypeId,
            config: targetSlot() ? { slot: targetSlot() } : {},
            values: {},
            blobs: { [portId]: { ...selection.blob } },
            execInput: 'in',
            execOutput: 'completed',
          },
        ],
        insertionPosition(dropPosition),
      )
      selectOnly(ids)
      options.assets.markUsed(selection.guid)
    } catch (error) {
      reject(error)
    }
  }

  function useResource(resource: WorkflowResource, variantId: string): void {
    placeResource(resource, variantId, false)
  }

  function locateBoundResource(location: ResourceLocation): void {
    options.showResourcePanel(location.kind)
    locateRequest.value = { ...location, requestId: ++locateSequence }
  }

  function importResource(resource: WorkflowResource, position?: { x: number; y: number }): void {
    const source = options.session.source
    if (!source) return
    const baseID = resource.id
    let id = baseID
    let suffix = 2
    while (source.resources.some((candidate) => candidate.id === id)) id = `${baseID}-${suffix++}`
    const snapshot = { ...copy(resource), id }
    const variantId = snapshot.kind === 'image' ? (snapshot.image?.variants[0]?.id ?? '') : ''
    placeResource(snapshot, variantId, true, position)
  }

  function placeResource(
    resource: WorkflowResource,
    variantId: string,
    addResource: boolean,
    requestedPosition?: { x: number; y: number },
  ): void {
    const portId =
      resource.kind === 'macro' ? 'macro' : resource.kind === 'input-clip' ? 'clip' : 'template'
    const binding = { resourceId: resource.id, ...(variantId ? { variantId } : {}) }
    const current = options.selectedNode.value
    const projection = current
      ? options.session.nodeProjection(current.nodeRef.nodeTypeId)
      : undefined
    if (current && projection?.dataInputs.some((port) => port.id === portId)) {
      try {
        if (addResource) {
          options.session.applyBatch([
            { kind: 'add-resource', resource },
            { kind: 'bind-resource', nodeId: current.id, portId, resource: binding },
          ])
        } else {
          options.session.apply({
            kind: 'bind-resource',
            nodeId: current.id,
            portId,
            resource: binding,
          })
        }
        return
      } catch (error) {
        reject(error)
        return
      }
    }
    const nodeTypeId =
      resource.kind === 'macro'
        ? 'https://schemas.yotta.dev/nodes/automation/play-macro'
        : resource.kind === 'input-clip'
          ? 'https://schemas.yotta.dev/nodes/automation/play-input-clip'
          : 'https://schemas.yotta.dev/nodes/automation/click-template'
    try {
      const ids = options.session.insertLinearDraft(
        [
          {
            nodeTypeID: nodeTypeId,
            config: targetSlot() ? { slot: targetSlot() } : {},
            values: {},
            blobs: {},
            resources: { [portId]: binding },
            execInput: 'in',
            execOutput: 'completed',
          },
        ],
        insertionPosition(requestedPosition),
        addResource ? [resource] : [],
      )
      selectOnly(ids)
    } catch (error) {
      reject(error)
    }
  }

  function updateResources(
    payloads: Array<{
      resourceId: string
      name: string
      description: string
      category: string
      tags: string[]
    }>,
  ): void {
    try {
      options.session.applyBatch(
        payloads.map((payload) => ({ kind: 'update-resource-metadata' as const, ...payload })),
      )
    } catch (error) {
      reject(error)
    }
  }

  function removeResources(resourceIds: string[]): void {
    try {
      options.session.applyBatch(
        resourceIds.map((resourceId) => ({ kind: 'remove-resource' as const, resourceId })),
      )
    } catch (error) {
      reject(error)
    }
  }

  function insertionPosition(requested?: { x: number; y: number }): { x: number; y: number } {
    if (requested) return requested
    const rect = options.canvasElement.value?.getBoundingClientRect()
    return rect
      ? options.screenToFlowCoordinate({
          x: rect.left + rect.width / 2,
          y: rect.top + rect.height / 2,
        })
      : { x: 160, y: 160 }
  }

  function targetSlot(): string {
    return (
      options.defaultTargetSlot.value ||
      options.recordingTargetSlot() ||
      captureTargetSlot.value ||
      options.recordingTargetItems.value[0]?.value ||
      ''
    )
  }

  function selectOnly(ids: string[]): void {
    options.selectedNodeIds.value = new Set(ids)
    options.selectedNodeId.value = ids[0] ?? ''
  }

  function reject(error: unknown): void {
    options.showError(options.translate('workflow.toast.edit_rejected'), error)
  }

  return {
    locateRequest,
    captureOpen,
    captureTargetSlot,
    captureBusy,
    captureIntent,
    openCapture,
    openRecapture,
    captureForNode,
    captureWorkspaceTemplate,
    removeVariant,
    useWorkspaceResource,
    useResource,
    locateBoundResource,
    importResource,
    updateResources,
    removeResources,
  }
}

function copy<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}
