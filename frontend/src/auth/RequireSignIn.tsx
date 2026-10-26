import type { ReactNode } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/Button'
import { Skeleton } from '@/components/ui/Skeleton'
import { useSession } from '@/auth/session'

interface RequireSignInProps {
  children: ReactNode
  /** One-line reason shown to signed-out visitors, e.g. "to see your orders". */
  reason?: string
}

/**
 * Gate for pages that need an account (FR-004). Signed out visitors get a short explanation
 * and a Sign in action, never data; browsing pages are not wrapped (FR-003).
 */
export function RequireSignIn({ children, reason = 'to use this page' }: RequireSignInProps) {
  const { status } = useSession()
  const navigate = useNavigate()
  const location = useLocation()

  if (status === 'loading') {
    return (
      <main className="mx-auto max-w-2xl px-md py-lg sm:px-lg">
        <Skeleton className="h-40 w-full" label="Checking your session" />
      </main>
    )
  }

  if (status === 'signedOut') {
    return (
      <main className="mx-auto max-w-md px-md py-2xl text-center sm:px-lg">
        <h1 className="mb-sm text-2xl font-bold text-neutral-900">Sign in {reason}</h1>
        <p className="mb-lg text-neutral-600">
          You can keep browsing food nearby without an account. Signing in takes a moment and
          brings you back here.
        </p>
        <Button
          type="button"
          onClick={() =>
            navigate('/sign-in', {
              state: { returnTo: `${location.pathname}${location.search}` },
            })
          }
        >
          Sign in
        </Button>
      </main>
    )
  }

  return <>{children}</>
}
