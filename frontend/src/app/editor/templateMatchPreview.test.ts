import { describe, expect, it } from 'vitest'
import authoring from '../../../../contracts/node/current/builtin-authoring'
import type { Node, NodeProjection } from './EditorSession'
import { resolveTemplatePreview, templatePreviewRequest } from './templateMatchPreview'

const projection = authoring.body.nodes.find(
  (node) => node.nodeRef.nodeTypeId === 'https://schemas.yotta.dev/nodes/automation/wait-template',
) as NodeProjection
const template = { mediaType: 'image/png', digest: `sha256:${'a'.repeat(64)}`, size: 100 }

describe('template preview input bindings', () => {
  it('reports the specific input blocking preview', () => {
    const node: Node = {
      id: 'wait',
      nodeRef: projection.nodeRef,
      position: { x: 0, y: 0 },
      config: {},
      bindings: { template: { kind: 'blob', blob: template } },
    }
    expect(resolveTemplatePreview(node, projection, [], '').blockedField).toBe('target')
    expect(resolveTemplatePreview(node, projection, [], 'game', new Set(['region']))).toMatchObject(
      { blockedField: 'region', dynamic: true },
    )
    node.bindings.threshold = { kind: 'value', value: 2 }
    expect(resolveTemplatePreview(node, projection, [], 'game')).toMatchObject({
      blockedField: 'threshold',
      request: null,
    })
    delete node.bindings.template
    expect(resolveTemplatePreview(node, projection, [], 'game').blockedField).toBe('template')
  })

  it('resolves explicit default bindings just like omitted default bindings', () => {
    const node: Node = {
      id: 'wait',
      nodeRef: projection.nodeRef,
      position: { x: 0, y: 0 },
      config: {},
      bindings: {
        template: { kind: 'blob', blob: template },
        threshold: { kind: 'default' },
        region: { kind: 'default' },
      },
    }
    const result = templatePreviewRequest(node, projection, [], 'window-target')
    expect(result).not.toBeNull()
    expect(result?.threshold).toBe(0.85)
    expect(result?.region).toEqual({ x: 0, y: 0, width: 1, height: 1, unit: 'ratio' })
    delete node.bindings.threshold
    delete node.bindings.region
    expect(templatePreviewRequest(node, projection, [], 'window-target')).toEqual(result)
  })
})
