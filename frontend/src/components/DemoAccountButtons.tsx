import { useState } from 'react'
import { demoSignIn, useDemoAccounts } from '@/services/accounts'
import { useSession } from '@/auth/session'
import { Button } from '@/components/ui/Button'

interface DemoAccountButtonsProps {
  /** Called after the demo session is active, e.g. to navigate back. */
  onSignedIn?: () => void
}

/**
 * One-tap demo accounts (FR-016). Renders nothing unless GET /api/v1/demo/accounts
 * responds, so production builds and backends without the demo issuer show no buttons.
 */
export function DemoAccountButtons({ onSignedIn }: DemoAccountButtonsProps) {
  const { data: accounts = [] } = useDemoAccounts()
  const session = useSession()
  const [pendingKey, setPendingKey] = useState<string | null>(null)
  const [failed, setFailed] = useState(false)

  if (accounts.length === 0) return null

  async function handle(key: string) {
    setPendingKey(key)
    setFailed(false)
    try {
      const token = await demoSignIn(key)
      session.signInWithDemoToken(token.access_token, token.expires_at)
      onSignedIn?.()
    } catch {
      setFailed(true)
    } finally {
      setPendingKey(null)
    }
  }

  return (
    <section data-testid="demo-accounts" aria-labelledby="demo-accounts-title">
      <h2 id="demo-accounts-title" className="mb-xs text-sm font-semibold text-neutral-700">
        Try a demo account
      </h2>
      <div className="flex flex-col gap-sm">
        {accounts.map((a) => (
          <Button
            key={a.key}
            type="button"
            variant="secondary"
            disabled={pendingKey !== null}
            onClick={() => void handle(a.key)}
          >
            {pendingKey === a.key ? 'Signing in…' : a.label}
          </Button>
        ))}
      </div>
      {failed && (
        <p role="alert" className="mt-sm text-sm text-neutral-700">
          Demo sign-in didn&apos;t work. Please try again.
        </p>
      )}
    </section>
  )
}
