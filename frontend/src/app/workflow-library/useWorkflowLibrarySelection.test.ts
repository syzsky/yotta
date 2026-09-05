import { describe, expect, it } from 'vitest'
import { ref } from 'vue'
import type { SourceView } from '@/app/transport/workflow'
import { useWorkflowLibrarySelection } from './useWorkflowLibrarySelection'

const source = (workflowId: string, name = workflowId) =>
  ({ workflowId, name, revision: 1, sourceHash: workflowId }) as SourceView

describe('useWorkflowLibrarySelection', () => {
  it('preserves explicit selection across pages while current-page select stays bounded', () => {
    const page = ref([source('a'), source('b')])
    const selection = useWorkflowLibrarySelection(page)
    selection.toggle(page.value[0]!, true)
    page.value = [source('c'), source('d')]
    selection.toggleCurrentPage(true)

    expect(selection.rows.value.map((item) => item.workflowId)).toEqual(['a', 'c', 'd'])
    expect(selection.allCurrentPageSelected.value).toBe(true)
    selection.toggleCurrentPage(false)
    expect(selection.rows.value.map((item) => item.workflowId)).toEqual(['a'])
  })

  it('can retain failures and remove successful results without leaking stale rows', () => {
    const page = ref([source('a'), source('b'), source('c')])
    const selection = useWorkflowLibrarySelection(page)
    selection.toggleCurrentPage(true)
    selection.retainOnly(['b', 'c'])
    selection.remove(['b'])
    expect(selection.rows.value.map((item) => item.workflowId)).toEqual(['c'])
    expect(selection.name('missing')).toBe('missing')
  })
})
