/**
 * Only same-origin paths may be used as a post-sign-in destination: a single leading '/'.
 * Anything else (absolute URLs, protocol-relative `//host`, `/\host`) falls back to '/', so a
 * crafted returnTo cannot send a freshly signed-in user to another site.
 */
export function safeReturnTo(value: unknown): string {
  if (typeof value !== 'string') return '/'
  if (!value.startsWith('/') || value.startsWith('//') || value.startsWith('/\\')) return '/'
  return value
}
