import { createApp, defineComponent, h, nextTick } from 'vue'
import { describe, expect, it } from 'vitest'
import { createI18n } from 'vue-i18n'
import {
  RunView,
  FailureView,
} from '@bindings/github.com/yottaapp/yotta/internal/services/workflow/models'
import zh from '@/i18n/zh'
import en from '@/i18n/en'
import RunTimelinePanel from './RunTimelinePanel.vue'

describe('persisted panel failures in the timeline', () => {
  it.each([
    ['zh', undefined, '面板操作未完成'],
    ['zh', 'component_missing', '重新选择'],
    ['en', 'component_missing', 'select it again'],
    ['zh', 'component_conflict', '不同的组件标识'],
    ['zh', 'invalid_value', '检查输入类型'],
  ])('renders a recovery message in %s for %s', async (locale, reason, expected) => {
    const root = document.createElement('div')
    document.body.append(root)
    const focused: unknown[] = []
    const run = new RunView({
      runId: 'run-evidence',
      status: 'failed',
      timeline: [],
      failure: new FailureView({
        code: reason ? `panels.${reason}` : 'panels.node_failed',
        category: 'adapter',
        graphId: 'main',
        nodeId: 'write',
        params: reason ? { reason, panel: 'main', component: 'value' } : {},
      }),
    })
    const app = createApp(RunTimelinePanel, {
      run,
      nodeLabels: { write: '设置已有开关值' },
      'onFocus-node': (...args: unknown[]) => focused.push(args),
    })
    app.component(
      'UButton',
      defineComponent({
        setup:
          (_, { slots }) =>
          () =>
            h('button', slots.default?.()),
      }),
    )
    app.use(createI18n({ legacy: false, locale, messages: { zh, en } }))
    try {
      app.mount(root)
      await nextTick()
      const alert = root.querySelector('[role="alert"]')!
      expect(alert.textContent).toContain(expected)
      expect(alert.textContent).not.toContain('panels.node_failed')
      expect(alert.textContent).not.toContain('adapter')
      alert.querySelector('button')!.click()
      expect(focused).toEqual([[['main'], 'write']])
      expect(root.textContent).toContain('run-evidence')
    } finally {
      app.unmount()
      root.remove()
    }
  })
})
