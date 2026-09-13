import { createApp, nextTick } from 'vue'
import ui from '@nuxt/ui/vue-plugin'
import { afterEach, expect, it } from 'vitest'
import type { FilterDimension } from '@bindings/github.com/yottaapp/yotta/internal/registryclient/models.js'
import WorkflowDimensionSelect from './WorkflowDimensionSelect.vue'

let app: ReturnType<typeof createApp> | undefined
afterEach(() => {
  app?.unmount()
  document.body.innerHTML = ''
})

function mount(assignment: boolean) {
  const dimensions = [
    { id: 'author', name: 'Author only', authorVisible: true, discoveryVisible: false },
    { id: 'search', name: 'Search only', authorVisible: false, discoveryVisible: true },
  ].map((d) => ({
    ...d,
    description: '',
    icon: '',
    active: true,
    position: 0,
    minValues: 0,
    maxValues: 0,
    maxDepth: 0,
    leafOnly: false,
    values: [
      {
        id: `${d.id}-parent`,
        name: 'Navigation',
        parentId: '',
        active: true,
        position: 0,
        assignable: false,
      },
      {
        id: `${d.id}-leaf`,
        name: 'Available leaf',
        parentId: `${d.id}-parent`,
        active: true,
        position: 1,
      },
    ],
  })) as FilterDimension[]
  const root = document.createElement('div')
  document.body.append(root)
  app = createApp(WorkflowDimensionSelect, { dimensions, assignment })
  app.use(ui)
  app.mount(root)
  return root
}

it('shows author dimensions and prevents assignment of navigation ancestors', async () => {
  const root = mount(true)
  await nextTick()
  expect(root.textContent).toContain('Author only')
  expect(root.textContent).not.toContain('Search only')
  const checks = root.querySelectorAll<HTMLButtonElement>('[role="checkbox"]')
  expect(checks).toHaveLength(2)
  expect(checks[0]?.disabled).toBe(true)
  expect(checks[1]?.disabled).toBe(false)
})

it('shows discovery dimensions and allows ancestor filtering', async () => {
  const root = mount(false)
  await nextTick()
  expect(root.textContent).toContain('Search only')
  expect(root.textContent).not.toContain('Author only')
  expect(root.querySelector<HTMLButtonElement>('[role="checkbox"]')?.disabled).toBe(false)
})
