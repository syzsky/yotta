import { ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { useWorkflowCanvasGestures } from './useWorkflowCanvasGestures'

describe('useWorkflowCanvasGestures', () => {
  it('keeps node and edge selection mutually exclusive', () => {
    const selectedNodeId = ref('')
    const selectedNodeIds = ref(new Set<string>())
    const selectedEdgeIds = ref(new Set(['edge-a']))
    const gestures = useWorkflowCanvasGestures({
      canvasElement: ref(null),
      selectedNodeId,
      selectedNodeIds,
      selectedEdgeIds,
      inspectorAutoOpen: ref(true),
      getSelectedNodes: () => [],
      findNode: () => undefined,
      addSelectedNodes: vi.fn(),
      removeSelectedNodes: vi.fn(),
      getViewport: () => ({ x: 0, y: 0, zoom: 1 }),
      setViewport: vi.fn(),
      closeConnectionMenu: vi.fn(),
      executeSelectionClear: vi.fn(),
      showNodeInspector: vi.fn(),
      dragPositions: () => [],
      trackLivePosition: vi.fn(),
      updateLivePosition: vi.fn(),
      applyPositions: vi.fn(),
      clearLivePosition: vi.fn(),
      clearGuides: vi.fn(),
    })

    gestures.selectNode({ node: { id: 'node-a' }, event: new MouseEvent('click') } as never)

    expect(selectedNodeId.value).toBe('node-a')
    expect([...selectedNodeIds.value]).toEqual(['node-a'])
    expect(selectedEdgeIds.value.size).toBe(0)
  })
})
