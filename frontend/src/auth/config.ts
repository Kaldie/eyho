export interface OidcEnvConfig {
  authority: string
  clientId: string
  audience: string | undefined
}

/**
 * OIDC settings from `VITE_OIDC_*`. Returns null when authority/client id are not set, in
 * which case the app runs without redirect sign-in (demo sign-in may still be available).
 */
export function getOidcConfig(): OidcEnvConfig | null {
  const authority = import.meta.env.VITE_OIDC_AUTHORITY as string | undefined
  const clientId = import.meta.env.VITE_OIDC_CLIENT_ID as string | undefined
  if (!authority || !clientId) return null
  return {
    authority,
    clientId,
    audience: (import.meta.env.VITE_OIDC_AUDIENCE as string | undefined) || undefined,
  }
}
