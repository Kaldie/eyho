import { Link, Navigate } from 'react-router-dom'
import { useSession } from '@/auth/session'
import { Skeleton } from '@/components/ui/Skeleton'

/**
 * Route for `/auth/callback`. react-oidc-context completes the code exchange on load and
 * `AuthProvider` then navigates back to the page that started sign-in; this page only
 * covers the waiting, error and "opened by hand" states.
 */
export function AuthCallbackPage() {
  const { status, error } = useSession()

  if (error) {
    return (
      <main className="mx-auto max-w-[28rem] px-md py-2xl text-center">
        <h1 className="mb-sm text-2xl font-bold text-neutral-900">Sign-in didn&apos;t complete</h1>
        <p className="mb-lg text-neutral-600">Something went wrong while signing you in.</p>
        <Link to="/sign-in" className="font-semibold text-primary-700">
          Try again
        </Link>
      </main>
    )
  }

  if (status === 'loading') {
    return (
      <main className="mx-auto max-w-[28rem] px-md py-2xl">
        <Skeleton className="h-24 w-full" label="Signing you in" />
      </main>
    )
  }

  return <Navigate to="/" replace />
}
