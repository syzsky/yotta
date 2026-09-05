import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import type { Connection, OnConnectStartParams } from '@vue-flow/core'
import type { EditorCommand, EditorSession } from './EditorSession'
import { graphHandle } from './graphHandles'
import { useWorkflowConnectionAuthoring } from './useWorkflowConnectionAuthoring'

function createFixture() {
  const commands: EditorCommand[] = []
  const session = {
    currentGraph: { nodes: [], calls: [], edges: [] },
    connectionCompatibility: vi.fn(() => ({ valid: true })),
  } as unknown as EditorSession
  const canvas = document.createElement('div')
  canvas.getBoundingClientRect = () => ({ left: 10, top: 20, width: 800, height: 600 }) as DOMRect
  const authoring = useWorkflowConnectionAuthoring({
    session,
    canvasElement: ref(canvas),
    selectedNodeId: ref(''),
    selectedNodeIds: ref(new Set()),
    applyCommand: (command) => {
      commands.push(command)
      return true
    },
    addNode: vi.fn(),
    screenToFlowCoordinate: (point) => ({ x: point.x - 10, y: point.y - 20 }),
    uniqueStateName: (value) => value,
    translate: (key) => key,
    translationExists: () => false,
    showError: vi.fn(),
  })
  return { authoring, commands }
}

describe('useWorkflowConnectionAuthoring', () => {
  it('turns a compatible Vue Flow connection into one editor command', () => {
    const fixture = createFixture()
    fixture.authoring.connect({
      source: 'left',
      target: 'right',
      sourceHandle: graphHandle('exec', 'output', 'done'),
      targetHandle: graphHandle('exec', 'input', 'in'),
    } as Connection)

    expect(fixture.commands).toEqual([
      {
        kind: 'connect',
        edge: {
          channel: 'exec',
          from: { nodeId: 'left', portId: 'done' },
          to: { nodeId: 'right', portId: 'in' },
        },
      },
    ])
  })

  it('opens the bounded candidate menu when a drag ends on empty canvas', async () => {
    const fixture = createFixture()
    fixture.authoring.startConnection({
      nodeId: 'left',
      handleId: graphHandle('exec', 'output', 'done'),
    } as OnConnectStartParams)
    fixture.authoring.endConnection(new MouseEvent('mouseup', { clientX: 790, clientY: 590 }))
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(fixture.authoring.connectionMenu.value).toMatchObject({
      anchor: { nodeId: 'left' },
      flowPosition: { x: 780, y: 570 },
      canvasPosition: { x: 472, y: 176 },
    })
    fixture.authoring.dispose()
  })
})
