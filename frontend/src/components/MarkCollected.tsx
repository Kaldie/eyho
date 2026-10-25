import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ApiError, confirmPickup } from '@/services/api'
import type { OrderSummary } from '@/services/api'
import { Button } from '@/components/ui/Button'

/** A paid order whose pickup window is open and that is not yet collected (FR-012). */
export function canMarkCollected(order: OrderSummary, now = Date.now()): boolean {
  return (
    order.paymentStatus === 'paid' &&
    order.pickupStatus === 'pending' &&
    new Date(order.pickupWindow.start).getTime() <= now &&
    now <= new Date(order.pickupWindow.end).getTime()
  )
}

function describeError(err: unknown): string {
  if (err instanceof ApiError && err.status === 409) {
    return err.code === 'claim_conflict'
      ? 'The cook cancelled this listing, so it can’t be marked as collected. Your payment is refunded.'
      : 'This order can’t be marked as collected any more (it may already be collected or past its pickup window).'
  }
  if (err instanceof ApiError && err.status === 403) return 'Only the buyer or the cook can confirm pickup.'
  return 'We couldn’t save that. Please try again.'
}

/**
 * "Mark as collected" for the buyer on paid orders in their pickup window. Without it a claim
 * ends as a no-show and the rating prompt is never reached. The cook-side action needs a
 * list of claims per listing, which the API does not offer yet.
 */
export function MarkCollected({ order }: { order: OrderSummary }) {
  const queryClient = useQueryClient()
  const mutation = useMutation({
    mutationFn: () => confirmPickup(order.orderId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['orders'] }),
    onError: (err) => {
      // A 409 means the stored state differs from what we show; refresh it.
      if (err instanceof ApiError && err.status === 409) {
        void queryClient.invalidateQueries({ queryKey: ['orders'] })
      }
    },
  })

  if (!canMarkCollected(order)) return null
  return (
    <div>
      <Button
        type="button"
        variant="secondary"
        disabled={mutation.isPending}
        onClick={() => mutation.mutate()}
      >
        {mutation.isPending ? 'Saving…' : 'Mark as collected'}
      </Button>
      {mutation.isError && (
        <p role="alert" data-testid="collect-error" className="mt-sm text-sm text-neutral-800">
          {describeError(mutation.error)}
        </p>
      )}
    </div>
  )
}
