import { computed, ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import type { EditorSession } from './EditorSession'
import { useWorkflowResourceAuthoring } from './useWorkflowResourceAuthoring'

describe('useWorkflowResourceAuthoring', () => {
  it('assigns a collision-free resource id and inserts the resource with its node atomically', () => {
    const insertLinearDraft = vi.fn(
      (_draft: unknown, _position: unknown, _resources: Array<{ id: string }>) => ['node-new'],
    )
    const session = {
      source: { resources: [{ id: 'image' }, { id: 'image-2' }] },
      currentGraph: { nodes: [] },
      insertLinearDraft,
    } as unknown as EditorSession
    const selectedNodeId = ref('')
    const selectedNodeIds = ref(new Set<string>())
    const authoring = useWorkflowResourceAuthoring({
      session,
      assets: {} as never,
      canvasElement: ref(null),
      selectedNode: computed(() => null),
      selectedNodeId,
      selectedNodeIds,
      defaultTargetSlot: computed(() => 'desktop'),
      recordingTargetSlot: () => '',
      recordingTargetItems: computed(() => []),
      screenToFlowCoordinate: (position) => position,
      applyCommand: vi.fn(),
      selectNodeForContextMenu: vi.fn(),
      showResourcePanel: vi.fn(),
      openScreenPicker: vi.fn(),
      waitForPickerResult: vi.fn(),
      translate: (key) => key,
      showError: vi.fn(),
    })

    authoring.importResource({
      id: 'image',
      kind: 'image',
      name: 'Image',
      image: { variants: [{ id: 'variant', blob: { hash: 'hash' } }] },
    } as never)

    expect(insertLinearDraft).toHaveBeenCalledOnce()
    expect(insertLinearDraft.mock.calls[0]?.[2]?.[0]?.id).toBe('image-3')
    expect(selectedNodeId.value).toBe('node-new')
    expect([...selectedNodeIds.value]).toEqual(['node-new'])
  })
})
