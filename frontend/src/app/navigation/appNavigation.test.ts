import { describe, expect, it } from 'vitest'
import { buildAppNavigation } from './appNavigation'

const translate = (key: string) => key

describe('app navigation hierarchy', () => {
  it('treats the editor as Workflow context instead of a fourth primary destination', () => {
    const model = buildAppNavigation('workflow-edit', translate)

    expect(model.primary).toHaveLength(4)
    expect(model.primary.find((item) => item.key === 'workflows')?.active).toBe(true)
    expect(model.contextTitle).toBe('')
    expect(model.contextIcon).toBe('')
  })

  it('places the marketplace immediately after Schedules', () => {
    const model = buildAppNavigation('market', translate)

    expect(model.primary.map((item) => item.key)).toEqual([
      'workflows',
      'assets',
      'schedules',
      'market',
    ])
    expect(model.primary.find((item) => item.key === 'market')?.active).toBe(true)
  })

  it('keeps utility pages named in the draggable context area', () => {
    const model = buildAppNavigation('settings', translate)

    expect(model.primary.every((item) => !item.active)).toBe(true)
    expect(model.contextTitle).toBe('sidebar.settings')
    expect(model.contextIcon).toBe('i-tabler-settings')
  })
})
