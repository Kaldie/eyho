import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/services/api'
import { ME_QUERY_KEY, resendVerificationEmail } from '@/services/accounts'
import { useAccount } from '@/auth/useAccount'
import { Button } from '@/components/ui/Button'

type ResendState =
  | { kind: 'idle' }
  | { kind: 'sending' }
  | { kind: 'sent' }
  | { kind: 'rate_limited' }
  | { kind: 'error' }

/**
 * Shown to signed-in people whose email is not verified (FR-006). ClaimFlow and the cook
 * area render it instead of proceeding. Neutral styling: it explains a next step, it is not
 * an error. Renders nothing when signed out or verified.
 */
export function VerifyEmailBanner({
  className = '',
  force = false,
}: {
  className?: string
  /** Show even when the cached account says verified: the API just answered email_not_verified. */
  force?: boolean
}) {
  const { account, refetch } = useAccount()
  const queryClient = useQueryClient()
  const [resend, setResend] = useState<ResendState>({ kind: 'idle' })
  const [stillUnverified, setStillUnverified] = useState(false)

  if (!account || (account.email_verified && !force)) return null

  async function handleResend() {
    setResend({ kind: 'sending' })
    try {
      await resendVerificationEmail()
      setResend({ kind: 'sent' })
    } catch (err) {
      if (err instanceof ApiError && err.status === 429) setResend({ kind: 'rate_limited' })
      else if (err instanceof ApiError && err.code === 'already_verified') {
        await queryClient.invalidateQueries({ queryKey: ME_QUERY_KEY })
        setResend({ kind: 'idle' })
      } else setResend({ kind: 'error' })
    }
  }

  async function handleCheckAgain() {
    setStillUnverified(false)
    await refetch()
    setStillUnverified(true)
  }

  return (
    <section
      data-testid="verify-email-banner"
      aria-labelledby="verify-email-title"
      className={[
        'rounded-card border border-neutral-300 bg-neutral-50 p-md text-sm text-neutral-700',
        className,
      ]
        .filter(Boolean)
        .join(' ')}
    >
      <h2 id="verify-email-title" className="mb-xs text-base font-bold text-neutral-900">
        Verify your email to continue
      </h2>
      <p className="mb-sm">
        We sent a link to <strong>{account.email ?? 'your email address'}</strong>. Open it, then
        sign in again so eyho sees the change. Claiming and publishing need a verified email.
      </p>
      <div className="flex flex-wrap items-center gap-sm">
        <Button
          type="button"
          variant="secondary"
          disabled={resend.kind === 'sending'}
          onClick={() => void handleResend()}
        >
          {resend.kind === 'sending' ? 'Sending…' : 'Resend verification email'}
        </Button>
        <Button type="button" variant="secondary" onClick={() => void handleCheckAgain()}>
          I&apos;ve verified
        </Button>
      </div>
      <div aria-live="polite" className="mt-sm">
        {resend.kind === 'sent' && <p>Verification email sent. Check your inbox and spam folder.</p>}
        {resend.kind === 'rate_limited' && (
          <p>You can request one email per minute. Please wait a moment and try again.</p>
        )}
        {resend.kind === 'error' && (
          <p>We couldn&apos;t send the email right now. Please try again shortly.</p>
        )}
        {stillUnverified && resend.kind !== 'sent' && (
          <p>Still not verified. If you just verified, sign out and sign in again.</p>
        )}
      </div>
    </section>
  )
}
