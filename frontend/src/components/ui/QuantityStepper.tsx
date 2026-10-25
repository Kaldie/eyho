export interface QuantityStepperProps {
  value: number
  min?: number
  max: number
  onChange: (value: number) => void
  /** Accessible label for the numeric input, e.g. "Portions". */
  label: string
  id?: string
  disabled?: boolean
}

/**
 * Keyboard-accessible quantity stepper with +/- buttons and a bounded numeric input.
 * Extracted/generalized from the inline quantity logic previously in ClaimFlow.tsx.
 *
 * Enforces `min`/`max` bounds per data-model.md's validation rules (quantity must be a
 * positive integer <= maxClaimableQuantity).
 */
export function QuantityStepper({
  value,
  min = 1,
  max,
  onChange,
  label,
  id = 'quantity-stepper',
  disabled = false,
}: QuantityStepperProps) {
  function clamp(v: number) {
    if (Number.isNaN(v)) return min
    return Math.min(max, Math.max(min, v))
  }

  return (
    <div>
      <label htmlFor={id} className="mb-sm block font-semibold text-neutral-800">
        {label}
      </label>
      <div className="flex items-center gap-sm">
        <button
          type="button"
          onClick={() => onChange(clamp(value - 1))}
          disabled={disabled || value <= min}
          aria-label={`Decrease ${label.toLowerCase()}`}
          className="flex h-9 w-9 items-center justify-center rounded-card border border-neutral-300 text-lg font-semibold text-neutral-700 hover:bg-neutral-50 disabled:cursor-not-allowed disabled:opacity-50"
        >
          -
        </button>
        <input
          id={id}
          type="number"
          inputMode="numeric"
          min={min}
          max={max}
          value={value}
          disabled={disabled}
          onChange={(e) => onChange(clamp(parseInt(e.target.value, 10)))}
          className="h-9 w-14 rounded-card border border-neutral-300 text-center text-base"
          aria-label={label}
        />
        <button
          type="button"
          onClick={() => onChange(clamp(value + 1))}
          disabled={disabled || value >= max}
          aria-label={`Increase ${label.toLowerCase()}`}
          className="flex h-9 w-9 items-center justify-center rounded-card border border-neutral-300 text-lg font-semibold text-neutral-700 hover:bg-neutral-50 disabled:cursor-not-allowed disabled:opacity-50"
        >
          +
        </button>
        <span className="text-sm text-neutral-500">of {max} available</span>
      </div>
    </div>
  )
}
