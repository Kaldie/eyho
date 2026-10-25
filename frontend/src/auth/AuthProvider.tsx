import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { AuthProvider as OidcAuthProvider, useAuth } from 'react-oidc-context'
import {
  InMemoryWebStorage,
  WebStorageStateStore,
  type User,
  type UserManagerSettings,
} from 'oidc-client-ts'
import { setAccessTokenGetter } from '@/services/api'
import { getOidcConfig, type OidcEnvConfig } from '@/auth/config'
import {
  SessionContext,
  type ResumeClaimState,
  type Session,
  type SessionStatus,
  type SignInState,
} from '@/auth/session'

const DEMO_STORAGE_KEY = 'eyho.demo-session'

interface DemoSession {
  token: string
  /** epoch ms, or null when the issuer gave no expiry. */
  expiresAt: number | null
}

/**
 * Demo-issuer tokens are only ever minted in demo/local environments (FR-017), are
 * short-lived, and are kept in sessionStorage so a reload doesn't drop the one-tap session.
 * OIDC tokens never touch web storage: they stay in memory.
 */
function loadDemoSession(): DemoSession | null {
  try {
    const raw = window.sessionStorage.getItem(DEMO_STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as DemoSession
    if (parsed.expiresAt !== null && parsed.expiresAt <= Date.now()) return null
    return parsed
  } catch {
    return null
  }
}

function saveDemoSession(session: DemoSession | null): void {
  try {
    if (session) window.sessionStorage.setItem(DEMO_STORAGE_KEY, JSON.stringify(session))
    else window.sessionStorage.removeItem(DEMO_STORAGE_KEY)
  } catch {
    // Storage unavailable (private mode): the session just lasts until reload.
  }
}

function buildSettings(cfg: OidcEnvConfig): UserManagerSettings {
  return {
    authority: cfg.authority,
    client_id: cfg.clientId,
    redirect_uri: `${window.location.origin}/auth/callback`,
    post_logout_redirect_uri: window.location.origin,
    response_type: 'code', // Authorization Code + PKCE
    // offline_access gives a rotating refresh token, so silent renew needs no iframe.
    scope: 'openid profile email offline_access',
    extraQueryParams: cfg.audience ? { audience: cfg.audience } : undefined,
    automaticSilentRenew: true,
    // Tokens in memory only; only the short-lived redirect state uses sessionStorage.
    userStore: new WebStorageStateStore({ store: new InMemoryWebStorage() }),
    stateStore: new WebStorageStateStore({ store: window.sessionStorage }),
  }
}

interface BridgeProps {
  demo: DemoSession | null
  setDemo: (session: DemoSession | null) => void
  children: ReactNode
}

/** Shared session assembly: demo token wins over OIDC user when both exist. */
function SessionProvider({
  demo,
  setDemo,
  oidc,
  children,
}: BridgeProps & {
  oidc: {
    isLoading: boolean
    user: User | null | undefined
    isAuthenticated: boolean
    error: Error | undefined
    signinRedirect: (state: SignInState) => Promise<void>
    signoutRedirect: () => Promise<void>
    removeUser: () => Promise<void>
  } | null
}) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const token = demo?.token ?? (oidc?.isAuthenticated ? (oidc.user?.access_token ?? null) : null)
  // Assigned during render so children's first queries already see the token.
  const tokenRef = useRef<string | null>(null)
  tokenRef.current = token
  setAccessTokenGetter(() => tokenRef.current)

  let status: SessionStatus
  if (demo) status = 'signedIn'
  else if (oidc?.isLoading) status = 'loading'
  else status = oidc?.isAuthenticated ? 'signedIn' : 'signedOut'

  const signIn = useCallback(
    async (state: SignInState = { returnTo: '/' }) => {
      if (oidc) await oidc.signinRedirect(state)
      else navigate('/sign-in', { state })
    },
    [oidc, navigate],
  )

  const signOut = useCallback(async () => {
    const wasDemo = demo !== null
    setDemo(null)
    queryClient.removeQueries({ queryKey: ['me'] })
    queryClient.removeQueries({ queryKey: ['orders'] })
    if (wasDemo || !oidc) {
      await oidc?.removeUser()
      navigate('/')
      return
    }
    try {
      await oidc.signoutRedirect()
    } catch {
      // Provider without an end-session endpoint: drop the local session only.
      await oidc.removeUser()
      navigate('/')
    }
  }, [demo, oidc, setDemo, queryClient, navigate])

  const signInWithDemoToken = useCallback(
    (accessToken: string, expiresAt?: string) => {
      const parsed = expiresAt ? Date.parse(expiresAt) : NaN
      setDemo({ token: accessToken, expiresAt: Number.isNaN(parsed) ? null : parsed })
      void queryClient.invalidateQueries({ queryKey: ['me'] })
    },
    [setDemo, queryClient],
  )

  const session = useMemo<Session>(
    () => ({
      status,
      oidcAvailable: oidc !== null,
      error: oidc?.error?.message ?? null,
      signIn,
      signOut,
      signInWithDemoToken,
    }),
    [status, oidc, signIn, signOut, signInWithDemoToken],
  )

  return <SessionContext.Provider value={session}>{children}</SessionContext.Provider>
}

function OidcBridge(props: BridgeProps) {
  const auth = useAuth()
  const oidc = useMemo(
    () => ({
      isLoading: auth.isLoading,
      user: auth.user,
      isAuthenticated: auth.isAuthenticated,
      error: auth.error,
      signinRedirect: (state: SignInState) => auth.signinRedirect({ state }),
      signoutRedirect: () => auth.signoutRedirect(),
      removeUser: () => auth.removeUser(),
    }),
    // `auth` changes identity on every state change, which is exactly when we must update.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [auth],
  )
  return <SessionProvider {...props} oidc={oidc} />
}

/**
 * AuthProvider (T013) — OIDC Authorization Code + PKCE via react-oidc-context, configured
 * from `VITE_OIDC_*`. After the redirect back at `/auth/callback` it returns the user to
 * the page they came from and, when the sign-in began at Claim, tells that page to reopen
 * the claim dialog with the same quantity (FR-005).
 *
 * Must be rendered inside the Router and the QueryClientProvider. Without OIDC settings
 * the app still works signed out, and demo sign-in can supply a session.
 */
export function AuthProvider({ children }: { children: ReactNode }) {
  const navigate = useNavigate()
  const [demo, setDemoState] = useState<DemoSession | null>(loadDemoSession)
  const cfg = useMemo(() => getOidcConfig(), [])
  const settings = useMemo(() => (cfg ? buildSettings(cfg) : null), [cfg])

  const setDemo = useCallback((session: DemoSession | null) => {
    saveDemoSession(session)
    setDemoState(session)
  }, [])

  // Drop the demo session when its token expires.
  useEffect(() => {
    if (!demo || demo.expiresAt === null) return
    const ms = demo.expiresAt - Date.now()
    const timer = window.setTimeout(() => setDemo(null), Math.max(ms, 0))
    return () => window.clearTimeout(timer)
  }, [demo, setDemo])

  if (!settings) {
    return (
      <SessionProvider demo={demo} setDemo={setDemo} oidc={null}>
        {children}
      </SessionProvider>
    )
  }

  return (
    <OidcAuthProvider
      {...settings}
      onSigninCallback={(user) => {
        const state = user?.state as SignInState | undefined
        const routerState: ResumeClaimState | undefined = state?.listingId
          ? { resumeClaim: { listingId: state.listingId, quantity: state.quantity ?? 1 } }
          : undefined
        navigate(state?.returnTo ?? '/', { replace: true, state: routerState })
      }}
    >
      <OidcBridge demo={demo} setDemo={setDemo}>
        {children}
      </OidcBridge>
    </OidcAuthProvider>
  )
}
