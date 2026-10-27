import { useState, type FormEvent } from 'react'
import { useMutation } from '@tanstack/react-query'
import { ApiError, updateListing, validationMessage } from '@/services/api'
import type { ListingItem, ListingStatus, UpdateListingInput } from '@/services/api'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'

interface CookListingRowProps {
  listing: ListingItem
  /** Called with the server's updated listing after a successful PATCH. */
  onUpdated: (listing: ListingItem) => void
  /** Called when the API says the email is unverified (the dashboard shows the banner). */
  onEmailNotVerified?: () => void
}

type Panel = 'portions' | 'extend' | 'cancel' | 'publish' | null

const STATUS_LABEL: Record<ListingStatus | 'sold_out', string> = {
  draft: 'draft',
  published: 'published',
  expired: 'expired',
  cancelled: 'cancelled',
  sold_out: 'sold out',
}

function formatPrice(cents: number): string {
  return new Intl.NumberFormat('nl-NL', { style: 'currency', currency: 'EUR' }).format(cents / 100)
}

function formatWindow(start: string, end: string): string {
  const day = new Date(start).toLocaleDateString('en-GB', { day: 'numeric', month: 'short' })
  const time = (iso: string) =>
    new Date(iso).toLocaleTimeString('nl-NL', { hour: '2-digit', minute: '2-digit' })
  return `${day}, ${time(start)}–${time(end)}`
}

/** `datetime-local` value (local time, minute precision) for an ISO instant. */
function toLocalInput(iso: string): string {
  const d = new Date(iso)
  d.setMinutes(d.getMinutes() - d.getTimezoneOffset())
  return d.toISOString().slice(0, 16)
}

/** Plain-language message for a failed PATCH (listings-api.md). */
function describeUpdateError(err: unknown): string {
  if (err instanceof ApiError) {
    switch (err.code) {
      case 'portions_conflict':
        return "You can't raise the portions above what is not held by buyers' claims. Enter a lower number."
      case 'window_not_extended':
        return 'Pick a time later than the current end of the pickup window.'
      case 'window_in_past':
        return 'The pickup window has already ended. Choose a new end time to publish.'
      case 'listing_not_draft':
        return 'This listing is already published. Refresh to see its current state.'
      case 'listing_not_active':
        return 'Only a published listing can be cancelled. Refresh to see its current state.'
      case 'not_owner':
        return 'Only the cook who owns this listing can change it.'
      case 'cook_not_registered':
        return 'Publishing is paused until your business registration is active. See the registration card above.'
    }
    if (err.status === 404) return 'This listing no longer exists.'
    if (err.status === 422) return validationMessage(err)
    if (err.status === 409) return 'This listing changed in the meantime. Refresh and try again.'
    if (err.status === 403) return 'You are not allowed to change this listing.'
  }
  return "We couldn't save that change. Please try again."
}

/**
 * One listing on the cook dashboard with its management actions (T028, FR-010): edit
 * portions, extend the pickup window, and cancel. Cancelling asks for confirmation first;
 * refunds and buyer notifications happen server-side. Statuses are neutral information
 * (Principle I); the cook's own verification label comes from the dashboard notice.
 */
export function CookListingRow({ listing, onUpdated, onEmailNotVerified }: CookListingRowProps) {
  const [panel, setPanel] = useState<Panel>(null)
  const [portions, setPortions] = useState(String(listing.portions_remaining))
  const [windowEnd, setWindowEnd] = useState(toLocalInput(listing.pickup_window_end))
  const [localError, setLocalError] = useState<string | null>(null)

  const status = listing.status ?? 'published'
  const soldOut = status === 'published' && listing.portions_remaining === 0
  const canEdit = status === 'published' || status === 'draft'

  const mutation = useMutation({
    mutationFn: (input: UpdateListingInput) => updateListing(listing.id, input),
    onSuccess: (updated) => {
      // Keep the status we know if a PATCH response omits it (the list shape has none).
      // PATCH responses carry no portions_claimed; keep the last known value.
      onUpdated({
        ...listing,
        ...updated,
        status: updated.status ?? listing.status,
        portions_claimed: updated.portions_claimed ?? listing.portions_claimed,
      })
      setPanel(null)
    },
    onError: (err) => {
      if (err instanceof ApiError && err.code === 'email_not_verified') onEmailNotVerified?.()
      // An elapsed window can be fixed in the same request: offer the end-time field.
      if (err instanceof ApiError && err.code === 'window_in_past') setPanel('publish')
    },
  })

  function open(next: Panel) {
    mutation.reset()
    setLocalError(null)
    setPanel(next)
  }

  function savePortions(e: FormEvent) {
    e.preventDefault()
    const n = Number(portions)
    if (!Number.isInteger(n) || n < 0) {
      setLocalError('Enter a whole number of portions, 0 or more.')
      return
    }
    setLocalError(null)
    mutation.mutate({ portions_remaining: n })
  }

  function saveExtend(e: FormEvent) {
    e.preventDefault()
    const next = new Date(windowEnd)
    if (Number.isNaN(next.getTime()) || next.getTime() <= new Date(listing.pickup_window_end).getTime()) {
      setLocalError('Pick a time later than the current end of the pickup window.')
      return
    }
    setLocalError(null)
    mutation.mutate({ pickup_window_end: next.toISOString() })
  }

  function publish(e?: FormEvent) {
    e?.preventDefault()
    const input: UpdateListingInput = { status: 'published' }
    if (panel === 'publish') {
      const next = new Date(windowEnd)
      if (Number.isNaN(next.getTime()) || next.getTime() <= Date.now()) {
        setLocalError('Choose a pickup end time in the future.')
        return
      }
      setLocalError(null)
      input.pickup_window_end = next.toISOString()
    }
    mutation.mutate(input)
  }

  const error = localError ?? (mutation.isError ? describeUpdateError(mutation.error) : null)
  const claimed = listing.portions_claimed
  const total = listing.total_portions

  return (
    <li
      data-testid="cook-listing"
      data-status={status}
      className="rounded-card border border-neutral-200 bg-white p-md shadow-xs"
    >
      <div className="flex flex-wrap items-start justify-between gap-sm">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-sm">
            <span className="font-semibold text-neutral-900">{listing.food_description}</span>
            <Badge tone={status === 'published' && !soldOut ? 'success' : 'neutral'}>
              {STATUS_LABEL[soldOut ? 'sold_out' : status]}
            </Badge>
            <Badge data-testid="listing-verification" tone="neutral">
              {listing.cook.verification_status}
            </Badge>
          </div>
          <div className="mt-xs flex flex-wrap gap-md text-sm tabular-nums text-neutral-600">
            <span>{formatPrice(listing.reduced_price_cents)}</span>
            <span>
              {listing.portions_remaining} portion{listing.portions_remaining === 1 ? '' : 's'} left
            </span>
            {claimed !== undefined && total !== undefined && (
              <span data-testid="claimed-count">
                {claimed} / {total} claimed
              </span>
            )}
            <span>{formatWindow(listing.pickup_window_start, listing.pickup_window_end)}</span>
          </div>
        </div>
      </div>

      {status === 'draft' && panel === null && (
        <p className="mt-sm text-sm text-neutral-600">
          Draft: not visible to buyers until you publish it.
        </p>
      )}

      {canEdit && panel === null && (
        <div className="mt-md flex flex-wrap gap-sm">
          {status === 'draft' && (
            <Button type="button" disabled={mutation.isPending} onClick={() => publish()}>
              {mutation.isPending ? 'Publishing…' : 'Publish'}
            </Button>
          )}
          <Button type="button" variant="secondary" onClick={() => open('portions')}>
            Edit portions
          </Button>
          <Button type="button" variant="secondary" onClick={() => open('extend')}>
            Extend pickup
          </Button>
          {status === 'published' && (
            <Button type="button" variant="secondary" onClick={() => open('cancel')}>
              Cancel listing
            </Button>
          )}
        </div>
      )}

      {panel === 'publish' && (
        <form onSubmit={publish} className="mt-md flex flex-wrap items-end gap-sm">
          <label className="flex flex-col gap-xs text-sm font-medium text-neutral-700">
            New pickup end
            <input
              type="datetime-local"
              value={windowEnd}
              onChange={(e) => setWindowEnd(e.target.value)}
              className="rounded-card border border-neutral-300 px-sm py-xs text-base text-neutral-900"
            />
          </label>
          <Button type="submit" disabled={mutation.isPending}>
            {mutation.isPending ? 'Publishing…' : 'Publish with new end'}
          </Button>
          <Button type="button" variant="secondary" onClick={() => open(null)}>
            Close
          </Button>
        </form>
      )}

      {panel === 'portions' && (
        <form onSubmit={savePortions} className="mt-md flex flex-wrap items-end gap-sm">
          <label className="flex flex-col gap-xs text-sm font-medium text-neutral-700">
            Portions remaining
            <input
              type="number"
              min={0}
              value={portions}
              onChange={(e) => setPortions(e.target.value)}
              className="rounded-card border border-neutral-300 px-sm py-xs text-base text-neutral-900"
            />
          </label>
          <Button type="submit" disabled={mutation.isPending}>
            {mutation.isPending ? 'Saving…' : 'Save portions'}
          </Button>
          <Button type="button" variant="secondary" onClick={() => open(null)}>
            Close
          </Button>
        </form>
      )}

      {panel === 'extend' && (
        <form onSubmit={saveExtend} className="mt-md flex flex-wrap items-end gap-sm">
          <label className="flex flex-col gap-xs text-sm font-medium text-neutral-700">
            New pickup end
            <input
              type="datetime-local"
              value={windowEnd}
              onChange={(e) => setWindowEnd(e.target.value)}
              className="rounded-card border border-neutral-300 px-sm py-xs text-base text-neutral-900"
            />
          </label>
          <Button type="submit" disabled={mutation.isPending}>
            {mutation.isPending ? 'Saving…' : 'Extend pickup'}
          </Button>
          <Button type="button" variant="secondary" onClick={() => open(null)}>
            Close
          </Button>
        </form>
      )}

      {panel === 'cancel' && (
        <div
          role="group"
          aria-label="Confirm cancelling this listing"
          className="mt-md rounded-card border border-neutral-300 bg-neutral-50 p-md text-sm text-neutral-800"
        >
          <p className="mb-sm font-semibold">Cancel this listing?</p>
          <p className="mb-sm">
            Buyers who already paid are refunded automatically and notified. This can&apos;t be
            undone.
          </p>
          <div className="flex flex-wrap gap-sm">
            <Button
              type="button"
              disabled={mutation.isPending}
              onClick={() => mutation.mutate({ status: 'cancelled' })}
            >
              {mutation.isPending ? 'Cancelling…' : 'Yes, cancel listing'}
            </Button>
            <Button type="button" variant="secondary" onClick={() => open(null)}>
              Keep listing
            </Button>
          </div>
        </div>
      )}

      {error && (
        <p role="alert" data-testid="listing-action-error" className="mt-sm text-sm font-medium text-neutral-900">
          {error}
        </p>
      )}
    </li>
  )
}
