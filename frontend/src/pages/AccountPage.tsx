import { useState, type FormEvent } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/services/api'
import { ME_QUERY_KEY, deleteMe, updateMe } from '@/services/accounts'
import { useAccount } from '@/auth/useAccount'
import { useSession } from '@/auth/session'
import { Button } from '@/components/ui/Button'
import { Skeleton } from '@/components/ui/Skeleton'
import { TextField } from '@/components/ui/TextField'

/**
 * Explains why deletion was refused (409). Counts come straight from the contract's
 * `{"error": "active_claims" | "active_listings", "count": n}` body.
 */
export function deletionBlockedMessage(err: unknown): string {
  if (err instanceof ApiError && err.status === 409) {
    const count = typeof err.body.count === 'number' ? err.body.count : null
    if (err.code === 'active_claims') {
      return count === 1
        ? 'You have 1 claim waiting for pickup. Collect it first, then you can delete your account.'
        : `You have ${count ?? 'some'} claims waiting for pickup. Collect them first, then you can delete your account.`
    }
    if (err.code === 'active_listings') {
      return count === 1
        ? 'One of your listings has a claim waiting for pickup. Once it has been collected you can delete your account.'
        : `${count ?? 'Some of'} your listings have claims waiting for pickup. Once they have been collected you can delete your account.`
    }
  }
  return 'We couldn’t delete your account right now. Please try again shortly.'
}

/**
 * AccountPage (T030) — display name, sign out, and account deletion (FR-013) with a
 * confirmation step and explanations for the 409 blocks.
 */
export function AccountPage() {
  const { account, isLoading, isError } = useAccount()
  const session = useSession()
  const queryClient = useQueryClient()

  const [name, setName] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [saveMessage, setSaveMessage] = useState<string | null>(null)
  const [nameError, setNameError] = useState<string | null>(null)

  const [confirming, setConfirming] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  if (isLoading) {
    return (
      <main className="mx-auto max-w-[36rem] px-md py-lg sm:px-lg">
        <Skeleton className="h-48 w-full" label="Loading account" />
      </main>
    )
  }

  if (isError || !account) {
    return (
      <main className="mx-auto max-w-[36rem] px-md py-lg sm:px-lg">
        <p className="text-neutral-700">We couldn&apos;t load your account. Please refresh the page.</p>
      </main>
    )
  }

  const nameValue = name ?? account.display_name ?? ''

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    const trimmed = nameValue.trim()
    if (!trimmed) {
      setNameError('Enter a display name.')
      return
    }
    setNameError(null)
    setSaving(true)
    setSaveMessage(null)
    try {
      const updated = await updateMe({ display_name: trimmed })
      queryClient.setQueryData(ME_QUERY_KEY, updated)
      setName(null)
      setSaveMessage('Saved.')
    } catch {
      setNameError('We couldn’t save your name. Please try again.')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete() {
    setDeleting(true)
    setDeleteError(null)
    try {
      await deleteMe()
      await session.signOut()
    } catch (err) {
      setDeleteError(deletionBlockedMessage(err))
      setConfirming(false)
    } finally {
      setDeleting(false)
    }
  }

  return (
    <main className="mx-auto max-w-[36rem] px-md py-lg sm:px-lg">
      <h1 className="mb-lg text-2xl font-bold text-neutral-900">Account</h1>

      <section className="mb-xl rounded-card border border-neutral-200 bg-white p-md">
        <p className="mb-md text-sm text-neutral-600">
          Signed in as <strong className="text-neutral-900">{account.email ?? 'unknown email'}</strong>
          {!account.email_verified && ' (email not verified)'}
        </p>
        <form onSubmit={(e) => void handleSave(e)} noValidate>
          <TextField
            label="Display name"
            value={nameValue}
            error={nameError}
            onChange={(e) => {
              setName(e.target.value)
              setSaveMessage(null)
            }}
            className="mb-md"
          />
          <div className="flex items-center gap-md">
            <Button type="submit" disabled={saving}>
              {saving ? 'Saving…' : 'Save'}
            </Button>
            {saveMessage && (
              <span role="status" className="text-sm text-neutral-700">
                {saveMessage}
              </span>
            )}
          </div>
        </form>
      </section>

      <section className="mb-xl">
        <Button type="button" variant="secondary" onClick={() => void session.signOut()}>
          Sign out
        </Button>
      </section>

      <section aria-labelledby="delete-title" className="rounded-card border border-neutral-200 bg-white p-md">
        <h2 id="delete-title" className="mb-xs text-lg font-bold text-neutral-900">
          Delete account
        </h2>
        <p className="mb-md text-sm text-neutral-600">
          This erases your name, email{account.cook_profile ? ', phone, addresses and KvK details' : ''} and
          sign-in link.
          {account.cook_profile &&
            ' Your cook record stays as "Former cook" so past ratings and verification history remain accurate.'}{' '}
          This can&apos;t be undone.
        </p>

        {deleteError && (
          <div
            role="alert"
            data-testid="delete-blocked"
            className="mb-md rounded-card border border-neutral-300 bg-neutral-50 px-md py-sm text-sm text-neutral-800"
          >
            {deleteError}
          </div>
        )}

        {!confirming ? (
          <Button type="button" variant="secondary" onClick={() => setConfirming(true)}>
            Delete my account
          </Button>
        ) : (
          <div role="alertdialog" aria-label="Confirm account deletion" className="flex flex-col gap-sm">
            <p className="text-sm font-semibold text-neutral-900">
              Delete your account permanently?
            </p>
            <div className="flex flex-wrap gap-sm">
              <Button type="button" disabled={deleting} onClick={() => void handleDelete()}>
                {deleting ? 'Deleting…' : 'Yes, delete my account'}
              </Button>
              <Button type="button" variant="secondary" onClick={() => setConfirming(false)}>
                Keep my account
              </Button>
            </div>
          </div>
        )}
      </section>
    </main>
  )
}
