import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import type { EditorCommand, EditorSession } from './EditorSession'
import { createEditorSelectionController } from './EditorSelectionController'

function createFixture() {
  const commands: EditorCommand[] = []
  const session = {
    currentGraph: {
      nodes: [{ id: 'node-a' }],
      calls: [{ id: 'call-a' }],
      annotations: [{ id: 'note-a' }],
    },
    duplicateNodes: vi.fn(() => []),
    selectionSnapshot: vi.fn(() => ({ nodes: [], calls: [], annotations: [], edges: [] })),
    insertNodeSelection: vi.fn(() => []),
  } as unknown as EditorSession
  const selectedNodeId = ref('')
  const selectedNodeIds = ref(new Set(['node-a', 'call-a', 'note-a']))
  const selectedEdgeIds = ref(new Set<string>())
  const controller = createEditorSelectionController({
    session,
    selectedNodeId,
    selectedNodeIds,
    selectedEdgeIds,
    selectedFlowNodes: () => [],
    findNode: () => undefined,
    addSelectedNodes: vi.fn(),
    removeSelectedNodes: vi.fn(),
    applyCommand: (command) => {
      commands.push(command)
      return true
    },
    disconnectEdge: vi.fn(),
    translate: (key) => key,
    showError: vi.fn(),
  })
  return {
    commands,
    controller,
    selectedEdgeIds,
    selectedNodeId,
    selectedNodeIds,
  }
}

describe('EditorSelectionController', () => {
  it('removes nodes, graph calls, and annotations as one user action', async () => {
    const { commands, controller, selectedNodeIds } = createFixture()

    await controller.execute({ kind: 'remove' })

    expect(commands.map((command) => command.kind)).toEqual([
      'remove-nodes',
      'remove-graph-call',
      'remove-annotation',
    ])
    expect(selectedNodeIds.value.size).toBe(0)
  })

  it('removes the selected edge when no canvas item is selected', async () => {
    const fixture = createFixture()
    fixture.selectedNodeIds.value = new Set()
    fixture.selectedEdgeIds.value = new Set(['edge-a', 'edge-b'])
    const disconnectEdge = vi.fn()
    const controller = createEditorSelectionController({
      session: {
        currentGraph: { nodes: [], calls: [], annotations: [] },
      } as unknown as EditorSession,
      selectedNodeId: fixture.selectedNodeId,
      selectedNodeIds: fixture.selectedNodeIds,
      selectedEdgeIds: fixture.selectedEdgeIds,
      selectedFlowNodes: () => [],
      findNode: () => undefined,
      addSelectedNodes: vi.fn(),
      removeSelectedNodes: vi.fn(),
      applyCommand: vi.fn(() => true),
      disconnectEdge,
      translate: (key) => key,
      showError: vi.fn(),
    })

    await controller.execute({ kind: 'remove' })

    expect(disconnectEdge).toHaveBeenNthCalledWith(1, 'edge-a')
    expect(disconnectEdge).toHaveBeenNthCalledWith(2, 'edge-b')
    expect(fixture.selectedEdgeIds.value.size).toBe(0)
  })

  it('clears both Vue Flow and editor selection state', async () => {
    const fixture = createFixture()
    const flowNodes = [{ id: 'node-a' }]
    const removeSelectedNodes = vi.fn()
    const controller = createEditorSelectionController({
      session: {} as EditorSession,
      selectedNodeId: ref('node-a'),
      selectedNodeIds: fixture.selectedNodeIds,
      selectedEdgeIds: ref(new Set(['edge-a'])),
      selectedFlowNodes: () => flowNodes,
      findNode: () => undefined,
      addSelectedNodes: vi.fn(),
      removeSelectedNodes,
      applyCommand: vi.fn(() => true),
      disconnectEdge: vi.fn(),
      translate: (key) => key,
      showError: vi.fn(),
    })

    await controller.execute({ kind: 'clear' })

    expect(removeSelectedNodes).toHaveBeenCalledWith(flowNodes)
    expect(fixture.selectedNodeIds.value.size).toBe(0)
  })

  it('supports single and additive edge selection', async () => {
    const fixture = createFixture()
    fixture.selectedNodeIds.value = new Set(['node-a'])

    await fixture.controller.execute({ kind: 'select-edge', edgeId: 'edge-a', additive: false })
    await fixture.controller.execute({ kind: 'select-edge', edgeId: 'edge-b', additive: true })
    expect(fixture.selectedEdgeIds.value).toEqual(new Set(['edge-a', 'edge-b']))
    expect(fixture.selectedNodeIds.value.size).toBe(0)

    await fixture.controller.execute({ kind: 'select-edge', edgeId: 'edge-a', additive: true })
    expect(fixture.selectedEdgeIds.value).toEqual(new Set(['edge-b']))
  })
})
