import { Navigate, useLocation, useNavigate } from 'react-router-dom'
import { safeReturnTo } from '@/auth/returnTo'
import { useSession, type SignInState } from '@/auth/session'
import { Button } from '@/components/ui/Button'
import { Skeleton } from '@/components/ui/Skeleton'
import { DemoAccountButtons } from '@/components/DemoAccountButtons'

/**
 * SignInPage (T034) — "Continue" starts the OIDC redirect (Google or email + password at
 * the provider; eyho never sees a password, FR-002). When the backend serves
 * /api/v1/demo/accounts, one-tap demo buttons appear too (FR-016).
 */
export function SignInPage() {
  const session = useSession()
  const navigate = useNavigate()
  const location = useLocation()
  const state = (location.state as Partial<SignInState> | null) ?? null
  const returnTo = safeReturnTo(state?.returnTo)

  if (session.status === 'signedIn') return <Navigate to={returnTo} replace />

  if (session.status === 'loading') {
    return (
      <main className="mx-auto max-w-[28rem] px-md py-lg">
        <Skeleton className="h-40 w-full" label="Checking your session" />
      </main>
    )
  }

  return (
    <main className="mx-auto max-w-[28rem] px-md py-2xl sm:px-lg">
      <h1 className="mb-sm text-2xl font-bold text-neutral-900">Sign in to eyho</h1>
      <p className="mb-lg text-neutral-600">
        Browsing is open to everyone. You only need an account to claim food, see your orders or
        cook.
      </p>

      {session.error && (
        <p role="alert" className="mb-md rounded-card border border-neutral-300 bg-neutral-50 px-md py-sm text-sm text-neutral-800">
          Sign-in didn&apos;t complete. Please try again.
        </p>
      )}

      {session.oidcAvailable && (
        <Button
          type="button"
          className="mb-lg w-full"
          onClick={() => void session.signIn({ ...state, returnTo })}
        >
          Continue
        </Button>
      )}

      <DemoAccountButtons onSignedIn={() => navigate(returnTo, { replace: true })} />

      {!session.oidcAvailable && (
        <p className="mt-md text-sm text-neutral-600">
          Sign-in isn&apos;t set up in this environment yet.
        </p>
      )}
    </main>
  )
}
