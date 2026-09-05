import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import type { YottaWorkflowSource } from '../../../../contracts/workflow/current/workflow-source'
import { useWorkflowNodeSearch } from './useWorkflowNodeSearch'

describe('useWorkflowNodeSearch', () => {
  it('searches labels, type titles and graph identity, then focuses through one interface', async () => {
    const source = ref({
      graphs: [
        {
          id: 'main',
          nodes: [
            {
              id: 'delay_1',
              label: '稍后执行',
              nodeRef: { nodeTypeId: 'control/delay', version: '1', semanticDigest: 'sha256:x' },
              position: { x: 0, y: 0 },
              config: {},
              bindings: {},
            },
          ],
        },
      ],
    } as unknown as YottaWorkflowSource)
    const focusNode = vi.fn(async () => undefined)
    const search = useWorkflowNodeSearch({
      source,
      nodeProjection: () => ({ icon: 'clock' }) as never,
      projectionTitle: () => '延时',
      focusNode,
    })

    search.show()
    search.query.value = '稍后'
    expect(search.results.value.map((result) => result.nodeId)).toEqual(['delay_1'])
    await search.select(search.results.value[0]!)
    expect(search.open.value).toBe(false)
    expect(focusNode).toHaveBeenCalledWith(['main'], 'delay_1')
  })
})
