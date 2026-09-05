import { ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import type { EditorSession, Node } from './EditorSession'
import { useWorkflowSnippetAuthoring } from './useWorkflowSnippetAuthoring'

describe('useWorkflowSnippetAuthoring', () => {
  it('builds an isolated snippet draft from a node', () => {
    const node = {
      id: 'node-a',
      nodeRef: { nodeTypeId: 'example/node', version: '1', semanticDigest: 'digest' },
      label: 'Example',
      position: { x: 0, y: 0 },
      config: { value: 'original' },
      bindings: {},
      disabled: false,
    } as Node
    const session = {
      nodeProjection: vi.fn(() => ({ category: 'test' })),
    } as unknown as EditorSession
    const authoring = useWorkflowSnippetAuthoring({
      session,
      snippets: {} as never,
      canvasElement: ref(null),
      screenToFlowCoordinate: (position) => position,
      selectInsertedNodes: vi.fn(),
      showSnippetPanel: vi.fn(),
      projectionTitle: () => 'Projected',
      confirm: vi.fn(),
      translate: (key) => key,
      showError: vi.fn(),
    })

    authoring.openForNode(node)
    node.config.value = 'changed'

    expect(authoring.modalOpen.value).toBe(true)
    expect(authoring.draft.value?.name).toBe('Example')
    expect(authoring.draft.value?.payload.config).toEqual({ value: 'original' })
  })
})
