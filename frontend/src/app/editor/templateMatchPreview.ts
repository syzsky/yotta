import type { Node, NodeProjection } from './EditorSession'
import type { WorkflowResource } from '../../../../contracts/workflow/current/workflow-source'
import type { TemplateMatchPreviewRequest } from '@/lib/backend'
import { resolveWorkflowResourceBinding } from './resourceLocator'

const templateNodes = new Set([
  'https://schemas.yotta.dev/nodes/vision/match-template',
  'https://schemas.yotta.dev/nodes/automation/wait-template',
  'https://schemas.yotta.dev/nodes/automation/click-template',
  'https://schemas.yotta.dev/nodes/automation/wait-template-gone',
])

export function supportsTemplatePreview(projection: NodeProjection | null): boolean {
  return Boolean(projection && templateNodes.has(projection.nodeRef.nodeTypeId))
}

export interface TemplatePreviewResolution {
  request: TemplateMatchPreviewRequest | null
  blockedField?: 'target' | 'template' | 'threshold' | 'region'
  dynamic?: boolean
}

export function resolveTemplatePreview(
  node: Node,
  projection: NodeProjection,
  resources: WorkflowResource[],
  targetSlot: string,
  connected: ReadonlySet<string> = new Set(),
): TemplatePreviewResolution {
  if (!targetSlot) return { request: null, blockedField: 'target' }
  for (const id of ['template', 'threshold', 'region'] as const) {
    if (connected.has(id)) return { request: null, blockedField: id, dynamic: true }
  }
  const binding = node.bindings.template
  const resource = resolveWorkflowResourceBinding(
    resources,
    binding?.kind === 'resource' ? binding.resource : undefined,
  )
  const template = binding?.kind === 'blob' ? binding.blob : resource?.blob
  if (!template) return { request: null, blockedField: 'template' }
  for (const id of ['threshold', 'region'] as const) {
    const value = node.bindings[id]
    if (value && value.kind !== 'value' && value.kind !== 'default')
      return { request: null, blockedField: id, dynamic: true }
  }
  const literal = (id: string): unknown => {
    const value = node.bindings[id]
    if (value && value.kind !== 'value' && value.kind !== 'default') return undefined
    return value?.kind === 'value'
      ? value.value
      : projection.dataInputs.find((port) => port.id === id)?.default
  }
  const threshold = literal('threshold')
  const region = literal('region') as TemplateMatchPreviewRequest['region'] | undefined
  if (
    typeof threshold !== 'number' ||
    !Number.isFinite(threshold) ||
    threshold < 0 ||
    threshold > 1
  )
    return { request: null, blockedField: 'threshold' }
  if (!region || !['px', 'ratio'].includes(region.unit))
    return { request: null, blockedField: 'region' }
  if (
    ![region.x, region.y, region.width, region.height].every(Number.isFinite) ||
    region.x < 0 ||
    region.y < 0 ||
    region.width <= 0 ||
    region.height <= 0
  )
    return { request: null, blockedField: 'region' }
  return {
    request: {
      targetSlot,
      template,
      threshold,
      region,
      variants: resource?.resource.image?.variants ?? [],
    },
  }
}

export function templatePreviewRequest(
  node: Node,
  projection: NodeProjection,
  resources: WorkflowResource[],
  targetSlot: string,
  connected?: ReadonlySet<string>,
): TemplateMatchPreviewRequest | null {
  return resolveTemplatePreview(node, projection, resources, targetSlot, connected).request
}
