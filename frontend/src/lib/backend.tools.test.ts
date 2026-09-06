import { describe, expect, it, vi } from 'vitest'

const tools = vi.hoisted(() => ({ loads: 0, preview: vi.fn(), mouse: vi.fn() }))
const ai = vi.hoisted(() => ({ loads: 0, status: vi.fn() }))
vi.mock('@bindings/github.com/yottaapp/yotta/internal/services/aiservice.js', () => {
  ai.loads++
  return { SecretStatus: ai.status }
})
vi.mock('@bindings/github.com/yottaapp/yotta/internal/services/tools/service.js', () => {
  tools.loads++
  return { PreviewTemplate: tools.preview, MousePos: tools.mouse }
})

import { backend, type TemplateMatchPreviewRequest } from './backend'

describe('lazy tools bridge', () => {
  it('loads the optional AI bridge on demand and forwards results', async () => {
    expect(ai.loads).toBe(0)
    ai.status.mockResolvedValue({ local: true })
    expect(await backend.ai.secretStatus(['local'])).toEqual({ local: true })
    expect(ai.status).toHaveBeenCalledWith(['local'])
    expect(ai.loads).toBe(1)
  })
  it('loads on use and forwards typed preview and existing tool calls', async () => {
    expect(tools.loads).toBe(0)
    const request: TemplateMatchPreviewRequest = {
      targetSlot: 'game',
      template: { mediaType: 'image/png', digest: `sha256:${'a'.repeat(64)}`, size: 8 },
      variants: [],
      region: { x: 0, y: 0, width: 1, height: 1, unit: 'ratio' },
      threshold: 0.85,
    }
    const result = { score: 0.82, matched: false, frameWidth: 1920, frameHeight: 1080 }
    tools.preview.mockResolvedValue(result)
    expect(await backend.tools.previewTemplate(request)).toEqual(result)
    expect(tools.preview).toHaveBeenCalledWith(request)
    tools.mouse.mockResolvedValue({ x: 1, y: 2 })
    expect(await backend.tools.mousePos('game')).toEqual({ x: 1, y: 2 })
    expect(tools.mouse).toHaveBeenCalledWith('game')
    expect(tools.loads).toBe(1)
  })
})
