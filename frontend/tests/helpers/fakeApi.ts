import { vi } from 'vitest'

/** One scripted response: status plus JSON body. */
export interface FakeResponse {
  status: number
  body?: unknown
}

export type Handler = FakeResponse | ((req: { url: string; method: string; body: unknown }) => FakeResponse)

/**
 * Replaces global fetch with a router keyed by "METHOD /path" (query string ignored).
 * Unscripted requests answer 404 so a missing route fails loudly instead of hanging.
 * Returns the mock for call assertions.
 */
export function stubApi(routes: Record<string, Handler>) {
  const mock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    const method = (init?.method ?? 'GET').toUpperCase()
    const key = `${method} ${url.split('?')[0]}`
    const handler = routes[key]
    const body = typeof init?.body === 'string' ? JSON.parse(init.body) : undefined
    const res: FakeResponse = !handler
      ? { status: 404, body: { error: 'not_found' } }
      : typeof handler === 'function'
        ? handler({ url, method, body })
        : handler
    return new Response(res.body === undefined ? null : JSON.stringify(res.body), {
      status: res.status,
      headers: { 'Content-Type': 'application/json' },
    })
  })
  vi.stubGlobal('fetch', mock)
  return mock
}

export function callsTo(mock: ReturnType<typeof stubApi>, method: string, path: string) {
  return mock.mock.calls.filter(
    ([url, init]) =>
      String(url).split('?')[0] === path && ((init as RequestInit | undefined)?.method ?? 'GET') === method,
  )
}
