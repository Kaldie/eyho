import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ApiError, rateClaim } from '@/services/api'
import { Button } from '@/components/ui/Button'

interface RatingPromptProps {
  claimId: string
  /** Existing rating (1-5) when the buyer already rated; null/undefined to prompt. */
  rating: number | null | undefined
}

function describeError(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 403) return "You can't rate this order. Only the buyer can rate it, and not on your own listing."
    if (err.status === 409 && err.code === 'not_collected') {
      return 'You can rate this order once pickup is confirmed.'
    }
    if (err.status === 422) return 'Pick a rating from 1 to 5 stars.'
    if (err.status === 401) return 'Your session has expired. Sign in again to rate.'
  }
  return "We couldn't save your rating. Please try again."
}

/**
 * Rating prompt for collected orders (FR-013, T025). One rating per claim: after a
 * 409 `already_rated` the prompt collapses to a thank-you instead of an error, since the
 * goal (a rating on file) is met. Ratings drive cook verification (Principle I), so the
 * copy says what the rating does without nudging toward a score.
 */
export function RatingPrompt({ claimId, rating }: RatingPromptProps) {
  const queryClient = useQueryClient()
  const [stars, setStars] = useState<number | null>(null)
  const [saved, setSaved] = useState<number | null>(null)
  const [alreadyRated, setAlreadyRated] = useState(false)

  const mutation = useMutation({
    mutationFn: (value: number) => rateClaim(claimId, value),
    onSuccess: (_data, value) => {
      setSaved(value)
      void queryClient.invalidateQueries({ queryKey: ['orders'] })
    },
    onError: (err) => {
      if (err instanceof ApiError && err.status === 409 && err.code === 'already_rated') {
        setAlreadyRated(true)
        void queryClient.invalidateQueries({ queryKey: ['orders'] })
      }
    },
  })

  const shown = saved ?? rating ?? null
  if (shown !== null) {
    return (
      <p data-testid="rating-done" className="text-sm text-neutral-700">
        You rated this order {shown} {shown === 1 ? 'star' : 'stars'}. Thank you.
      </p>
    )
  }
  if (alreadyRated) {
    return (
      <p data-testid="rating-done" role="status" className="text-sm text-neutral-700">
        You&apos;ve already rated this order. Thank you.
      </p>
    )
  }

  return (
    <fieldset data-testid="rating-prompt" className="rounded-card border border-neutral-200 bg-neutral-50 p-md">
      <legend className="px-xs text-sm font-semibold text-neutral-900">How was your pickup?</legend>
      <p className="mb-sm text-sm text-neutral-600">
        Your rating helps other buyers and decides whether this cook becomes verified.
      </p>
      <div role="radiogroup" aria-label="Rating" className="mb-sm flex flex-wrap gap-xs">
        {[1, 2, 3, 4, 5].map((n) => (
          <button
            key={n}
            type="button"
            role="radio"
            aria-checked={stars === n}
            aria-label={`${n} ${n === 1 ? 'star' : 'stars'}`}
            onClick={() => setStars(n)}
            className={[
              'min-h-[2.75rem] min-w-[2.75rem] rounded-control border text-base font-semibold focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-700',
              stars === n
                ? 'border-primary-700 bg-primary-600 text-white'
                : 'border-neutral-300 bg-white text-neutral-900 hover:bg-neutral-100',
            ].join(' ')}
          >
            {n}
          </button>
        ))}
      </div>
      <Button
        type="button"
        disabled={stars === null || mutation.isPending}
        onClick={() => stars !== null && mutation.mutate(stars)}
      >
        {mutation.isPending ? 'Sending…' : 'Send rating'}
      </Button>
      {mutation.isError && !alreadyRated && (
        <p role="alert" data-testid="rating-error" className="mt-sm text-sm text-neutral-800">
          {describeError(mutation.error)}
        </p>
      )}
    </fieldset>
  )
}
