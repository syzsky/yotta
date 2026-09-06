import { afterEach, expect, it } from 'vitest'
import { i18n, setLocale } from './index'

afterEach(async () => {
  await setLocale('zh')
})

it('does not let a pending English load override a later language selection', async () => {
  const pending = setLocale('en')
  await setLocale('zh')
  await pending
  expect(i18n.global.locale.value).toBe('zh')
})

it('loads English without discarding plugin-owned messages', async () => {
  i18n.global.mergeLocaleMessage('en', { plugin: { sample: { title: 'Example extension' } } })
  expect(await setLocale('en')).toBe(true)
  expect(i18n.global.locale.value).toBe('en')
  expect(i18n.global.t('plugin.sample.title')).toBe('Example extension')
  expect(i18n.global.t('common.cancel')).toBe('Cancel')
})
