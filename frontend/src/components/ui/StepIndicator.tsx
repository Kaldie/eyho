import type { CheckoutStep } from '@/types/views'

const STEPS: { key: CheckoutStep; label: string }[] = [
  { key: 'quantity', label: 'Portions' },
  { key: 'payment', label: 'Payment' },
  { key: 'confirmation', label: 'Confirmation' },
]

export interface StepIndicatorProps {
  step: CheckoutStep
  className?: string
}

/**
 * Renders progress through CheckoutState.step (FR-004) so the buyer always sees where
 * they are in the claim/checkout flow and can trust the flow is moving forward.
 */
export function StepIndicator({ step, className = '' }: StepIndicatorProps) {
  const currentIndex = STEPS.findIndex((s) => s.key === step)

  return (
    <ol
      aria-label="Checkout progress"
      className={['flex items-center gap-sm', className].filter(Boolean).join(' ')}
    >
      {STEPS.map((s, index) => {
        const isComplete = index < currentIndex
        const isCurrent = index === currentIndex
        return (
          <li key={s.key} className="flex flex-1 items-center gap-sm">
            <span
              aria-current={isCurrent ? 'step' : undefined}
              className={[
                'flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-xs font-semibold',
                isCurrent
                  ? 'bg-primary-600 text-white'
                  : isComplete
                    ? 'bg-primary-100 text-primary-700'
                    : 'bg-neutral-100 text-neutral-500',
              ].join(' ')}
            >
              {index + 1}
            </span>
            <span
              className={[
                'text-sm',
                isCurrent ? 'font-semibold text-neutral-900' : 'sr-only text-neutral-500 sm:not-sr-only',
              ].join(' ')}
            >
              {s.label}
            </span>
            {index < STEPS.length - 1 && (
              <span aria-hidden="true" className="h-px min-w-[1rem] flex-1 bg-neutral-200" />
            )}
          </li>
        )
      })}
    </ol>
  )
}
