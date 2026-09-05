import { computed, ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import type { Edge, EditorSession } from './EditorSession'
import { useWorkflowQuickAdd } from './useWorkflowQuickAdd'

describe('useWorkflowQuickAdd', () => {
  it('filters batch insertion candidates by every selected signal channel', () => {
    const edges = [{ channel: 'exec' }, { channel: 'error' }] as Edge[]
    const session = {
      nodeProjection: vi.fn(() => ({
        signals: [
          { direction: 'input', channel: 'exec' },
          { direction: 'output', channel: 'exec' },
          { direction: 'input', channel: 'error' },
          { direction: 'output', channel: 'error' },
        ],
      })),
    } as unknown as EditorSession
    const quickAdd = useWorkflowQuickAdd({
      session,
      snippets: { items: [] } as never,
      canvasElement: ref(null),
      catalogNodes: computed(
        () => [{ nodeRef: { nodeTypeId: 'node/type' }, category: 'logic', icon: 'box' }] as never,
      ),
      selectedEdgeIds: ref(new Set(['a', 'b'])),
      selectedNodeId: ref(''),
      selectedNodeIds: ref(new Set()),
      lastCanvasPointer: ref(null),
      selectedSourceEdges: () => edges,
      conversionNodePosition: () => ({ x: 0, y: 0 }),
      screenToFlowCoordinate: (position) => position,
      viewport: () => ({ x: 0, y: 0, zoom: 1 }),
      canvasAssistCollapsed: () => false,
      addNode: vi.fn(),
      useSnippet: vi.fn(),
      projectionTitle: () => 'Node',
      categoryLabel: () => 'Logic',
      catalogSearchText: () => 'node',
      translate: (key) => key,
      translationExists: () => false,
      showError: vi.fn(),
    })

    expect(quickAdd.insertableItems.value.map((item) => item.id)).toEqual(['node/type'])
  })
})
