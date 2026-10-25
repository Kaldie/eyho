import { describe, expect, it } from 'vitest'
import { safeReturnTo } from '@/auth/returnTo'

describe('safeReturnTo', () => {
  it('keeps same-origin paths', () => {
    expect(safeReturnTo('/')).toBe('/')
    expect(safeReturnTo('/listings/1?x=2')).toBe('/listings/1?x=2')
  })

  it('rejects anything that could leave the origin', () => {
    for (const bad of ['https://evil.example', '//evil.example', '/\\evil.example', 'javascript:alert(1)', 'listings', '', undefined, null, 42]) {
      expect(safeReturnTo(bad)).toBe('/')
    }
  })
})
