import { createContext, useContext } from 'react'

/**
 * Carried through the OIDC `state` parameter so the buyer returns to the exact action they
 * started (FR-005): the page, the listing and the chosen quantity.
 */
export interface SignInState {
  returnTo: string
  listingId?: string
  quantity?: number
}

/** Router state set after sign-in to reopen the claim dialog (read by discovery/detail pages). */
export interface ResumeClaimState {
  resumeClaim?: { listingId: string; quantity: number }
}

export type SessionStatus = 'loading' | 'signedOut' | 'signedIn'

export interface Session {
  status: SessionStatus
  /** True when redirect sign-in (OIDC) is configured for this build. */
  oidcAvailable: boolean
  /** Set when the sign-in callback failed. */
  error: string | null
  /** Starts OIDC sign-in, or goes to /sign-in when OIDC is not configured. */
  signIn: (state?: SignInState) => Promise<void>
  signOut: () => Promise<void>
  /** Adopts a token from the demo issuer as the session (demo/local only). */
  signInWithDemoToken: (accessToken: string, expiresAt?: string) => void
}

export const SIGNED_OUT_SESSION: Session = {
  status: 'signedOut',
  oidcAvailable: false,
  error: null,
  signIn: () => Promise.resolve(),
  signOut: () => Promise.resolve(),
  signInWithDemoToken: () => undefined,
}

export const SessionContext = createContext<Session>(SIGNED_OUT_SESSION)

export function useSession(): Session {
  return useContext(SessionContext)
}
