import { createApp, defineComponent, h, KeepAlive, nextTick, ref } from 'vue'
import { VueFlow, type VueFlowStore } from '@vue-flow/core'
import { expect, it } from 'vitest'
import { useWorkflowCanvasStore } from './useWorkflowCanvasStore'

it('keeps cached workflow nodes, edges and connection validation in their own canvas', async () => {
  const active = ref('a')
  const stores = new Map<string, VueFlowStore>()
  const Editor = defineComponent({
    props: { workflow: { type: String, required: true } },
    setup(props) {
      const id = props.workflow
      const flow = useWorkflowCanvasStore()
      stores.set(id, flow)
      return () =>
        h(VueFlow, {
          id: flow.id,
          nodes: [
            { id: `${id}-start`, position: { x: 0, y: 0 }, data: {} },
            { id: `${id}-end`, position: { x: 200, y: 0 }, data: {} },
          ],
          edges: [{ id: `${id}-edge`, source: `${id}-start`, target: `${id}-end` }],
          isValidConnection: (connection) => connection.source === `${id}-start`,
        })
    },
  })
  const root = document.createElement('div')
  document.body.append(root)
  const app = createApp({
    render: () =>
      h(
        KeepAlive,
        { max: 3 },
        {
          default: () => h(Editor, { key: active.value, workflow: active.value }),
        },
      ),
  })
  try {
    app.mount(root)
    await nextTick()
    active.value = 'b'
    await nextTick()
    active.value = 'a'
    await nextTick()
    expect(stores.get('a')!.getNodes.value.map((node) => node.id)).toEqual(['a-start', 'a-end'])
    expect(stores.get('a')!.getEdges.value.map((edge) => edge.id)).toEqual(['a-edge'])
    expect(stores.get('b')!.getNodes.value.map((node) => node.id)).toEqual(['b-start', 'b-end'])
    expect(
      stores.get('a')!.isValidConnection.value?.(
        {
          source: 'a-start',
          target: 'a-end',
          sourceHandle: null,
          targetHandle: null,
        },
        {
          nodes: stores.get('a')!.getNodes.value,
          edges: stores.get('a')!.getEdges.value,
          sourceNode: stores.get('a')!.findNode('a-start')!,
          targetNode: stores.get('a')!.findNode('a-end')!,
        },
      ),
    ).toBe(true)
    // Exceed the App's three-editor cache: evicting A must not destroy B's store.
    for (const id of ['c', 'd', 'b']) {
      active.value = id
      await nextTick()
    }
    expect(stores.get('b')!.getNodes.value.map((node) => node.id)).toEqual(['b-start', 'b-end'])
    expect(stores.get('b')!.getEdges.value.map((edge) => edge.id)).toEqual(['b-edge'])
    expect(root.querySelector('[data-id="b-start"]')).not.toBeNull()
    expect(root.querySelector('[data-id="a-start"]')).toBeNull()
  } finally {
    app.unmount()
    root.remove()
  }
})
