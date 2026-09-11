import { createApp } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import PanelRenderer from './PanelRenderer.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key, locale: { value: 'en' } }),
}))
vi.mock('@/lib/workflowIcons', () => ({ ensureWorkflowIcons: async () => undefined }))

describe('panel timer', () => {
  it('shows an unstarted timer as zero instead of time since the Unix epoch', () => {
    const root = document.createElement('div')
    const app = createApp(PanelRenderer, {
      components: [{ id: 'timer', kind: 'timer', field: 'timer', titleKey: 'Timer' }],
      snapshot: {
        protocol: 'yotta.panel-provider/v1',
        sessionId: 'test',
        revision: 1,
        status: 'ready',
        values: { timer: 0 },
        controlRevisions: {},
        records: {},
      },
      disabled: false,
      busy: '',
      now: 1789090000000,
      columns: 3,
    })
    app.config.warnHandler = () => undefined
    app.mount(root)
    try {
      expect(root.textContent).toContain('00:00:00')
    } finally {
      app.unmount()
    }
  })
})
