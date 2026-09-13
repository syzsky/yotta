import { createApp, nextTick } from 'vue'
import { createI18n } from 'vue-i18n'
import { createMemoryHistory, createRouter } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { afterEach, expect, it } from 'vitest'
import enNode from '@/i18n/locales/en/node'
import zhNode from '@/i18n/locales/zh/node'
import enWorkflow from '@/i18n/locales/en/workflow'
import zhWorkflow from '@/i18n/locales/zh/workflow'
import enResources from '@/i18n/locales/en/resources'
import zhResources from '@/i18n/locales/zh/resources'
import type { FieldProjection } from '@/contracts/node'
import PathFollowingHelp from './PathFollowingHelp.vue'
import GeneratedFieldEditor from './GeneratedFieldEditor.vue'
import PathPointNameField from '@/app/paths/PathPointNameField.vue'
import RunTimelinePanel from './RunTimelinePanel.vue'
import {
  RunView,
  TimelineEntry,
} from '@bindings/github.com/yottaapp/yotta/internal/services/workflow/models'

let app: ReturnType<typeof createApp> | undefined
afterEach(() => {
  app?.unmount()
  document.body.innerHTML = ''
})

function mount(
  component: Parameters<typeof createApp>[0],
  props: Record<string, unknown>,
  locale = 'en',
) {
  const root = document.createElement('div')
  document.body.append(root)
  app = createApp(component, props)
  app.use(ui)
  app.use(
    createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }],
    }),
  )
  app.use(
    createI18n({
      legacy: false,
      locale,
      messages: {
        en: { ...enNode, ...enWorkflow, ...enResources },
        zh: { ...zhNode, ...zhWorkflow, ...zhResources },
      },
    }),
  )
  app.mount(root)
  return root
}

it.each(['follow-path', 'follow-saved-path'])('offers action wiring help on %s', async (name) => {
  const root = mount(PathFollowingHelp, {
    nodeTypeId: `https://schemas.yotta.dev/nodes/navigation/${name}`,
  })
  expect(root.textContent).toContain('existing key nodes or subgraph calls')
  const details = root.querySelector('details')!
  expect(details.open).toBe(false)
  root.querySelector('summary')!.click()
  await nextTick()
  expect(details.open).toBe(true)
  expect(details.textContent).toContain('never overlap')
  expect(details.textContent).toContain('Point name output')
  expect(details.textContent).toContain('resumes automatically')
  expect(details.textContent).toContain('terminal output')
  expect(details.textContent).toContain('0 to 1')
  expect(details.textContent).toContain('Started output to the follower input')
  expect(root.querySelector('textarea')).toBeNull()
})

it('does not add path help to unrelated nodes', () => {
  const root = mount(PathFollowingHelp, {
    nodeTypeId: 'https://schemas.yotta.dev/nodes/control/delay',
  })
  expect(root.querySelector('section')).toBeNull()
})

it.each(['en', 'zh'])(
  'edits and clears a marker through the existing point name in %s',
  async (locale) => {
    const values: string[] = []
    const root = mount(
      PathPointNameField,
      {
        modelValue: 'Entrance',
        'onUpdate:modelValue': (value: string) => values.push(value),
      },
      locale,
    )
    expect(root.textContent).toContain('F6')
    const input = root.querySelector('input')!
    expect(input.value).toBe('Entrance')
    expect(input.getAttribute('aria-label')).toBe(locale === 'en' ? 'Point name' : '点名称')
    input.value = 'Exit'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()
    input.value = ''
    input.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()
    expect(values).toEqual(['Exit', ''])
  },
)

it('disables marker naming while recording', () => {
  const root = mount(PathPointNameField, { modelValue: '', disabled: true })
  expect(root.querySelector('input')!.disabled).toBe(true)
})

it.each(['en', 'zh'])(
  'shows translated marker modes and emits contract values in %s',
  async (locale) => {
    const field: FieldProjection = {
      id: 'marker-mode',
      titleKey: 'node.navigation.followPath.markerMode',
      constraints: { enum: ['continue', 'pause'] },
      control: 'select',
      default: 'continue',
      hasDefault: true,
      deprecated: false,
      examples: [],
      properties: [],
      readOnly: false,
      required: false,
    }
    const values: unknown[] = []
    const root = mount(
      GeneratedFieldEditor,
      {
        field,
        modelValue: 'continue',
        'onUpdate:modelValue': (value: unknown) => values.push(value),
      },
      locale,
    )
    const select = root.querySelector<HTMLButtonElement>('[role="combobox"]')!
    expect(select.textContent).toContain(locale === 'en' ? 'Continue moving' : '继续移动')
    select.click()
    await nextTick()
    const option = Array.from(document.querySelectorAll<HTMLElement>('[role="option"]')).find(
      (el) => el.textContent?.includes(locale === 'en' ? 'Pause for action' : '等待动作完成'),
    )!
    expect(option).toBeDefined()
    option.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, pointerType: 'mouse' }))
    option.dispatchEvent(new PointerEvent('pointerup', { bubbles: true, pointerType: 'mouse' }))
    await nextTick()
    await nextTick()
    expect(values).toEqual(['pause'])
  },
)

it.each(['en', 'zh'])('explains distinct path statuses in the timeline in %s', (locale) => {
  const statuses = [
    'recovering',
    'stuck',
    'turning-stuck',
    'unavailable',
    'reference-mismatch',
    'height-mismatch',
  ]
  const root = mount(
    RunTimelinePanel,
    {
      run: new RunView({
        runId: 'fixture-path',
        status: 'SUCCEEDED',
        timeline: statuses.map(
          (status, index) =>
            new TimelineEntry({
              sequence: index + 1,
              kind: 'status',
              nodeId: 'follower',
              statusCode: `navigation.path.${status}`,
              occurredAt: '2026-01-01T00:00:00Z',
            }),
        ),
      }),
    },
    locale,
  )
  const labels = (locale === 'en' ? enWorkflow : zhWorkflow).workflow.timeline.status
  for (const status of statuses) {
    expect(root.textContent).toContain(labels[`navigation.path.${status}` as keyof typeof labels])
  }
})
