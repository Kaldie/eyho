import { useMemo, useRef, useState } from 'react'
import { Button } from '@/components/ui/Button'
import { ListingCard } from '@/components/ListingCard'
import type { ListingItem, CookSummary } from '@/services/api'

export interface ListingFormValues {
  foodDescription: string
  allergenInfo: string
  reducedPriceCents: number
  /** What the dish would cost normally; the reduced price must stay below it (FR-003). */
  retailReferencePriceCents: number
  portionsTotal: number
  pickupWindowStart: string
  pickupWindowEnd: string
  photoUrl: string
}

/** A validation problem; `field` is the contract's snake_case field name when known. */
export interface FormError {
  message: string
  field?: string
}

export interface ListingFormProps {
  /** The signed-in cook's own profile — used to build the live preview card. */
  cook: CookSummary
  initialValues?: Partial<ListingFormValues>
  /** `publish` false saves a draft, which unregistered cooks may still do. */
  onSubmit?: (values: ListingFormValues, publish: boolean) => void
  onCancel?: () => void
  submitting?: boolean
  /** Server (422) or client validation error, shown next to the field it names. */
  error?: FormError | null
  /** Shown as the pickup location; the API falls back to it when no address is sent. */
  defaultPickupAddress?: string
}

const EMPTY_VALUES: ListingFormValues = {
  foodDescription: '',
  allergenInfo: '',
  reducedPriceCents: 0,
  retailReferencePriceCents: 0,
  portionsTotal: 1,
  pickupWindowStart: '',
  pickupWindowEnd: '',
  photoUrl: '',
}

const INPUT_CLASS =
  'rounded-card border border-neutral-300 px-sm py-xs text-base text-neutral-900 aria-[invalid=true]:border-neutral-900'

/** Client-side checks mirroring the contract's 422s (the server stays authoritative). */
export function validateListing(values: ListingFormValues, now = Date.now()): FormError | null {
  if (values.reducedPriceCents >= values.retailReferencePriceCents) {
    return {
      field: 'reduced_price_cents',
      message: 'The reduced price must be lower than the normal price.',
    }
  }
  const start = new Date(values.pickupWindowStart).getTime()
  const end = new Date(values.pickupWindowEnd).getTime()
  if (Number.isNaN(start) || Number.isNaN(end) || end <= start) {
    return { field: 'pickup_window_end', message: 'Pickup must end after it starts.' }
  }
  if (end <= now) {
    return { field: 'pickup_window_end', message: 'The pickup window must end in the future.' }
  }
  return null
}

/**
 * ListingForm — creation/edit form for a cook's listing, with a live-updating
 * preview of the buyer-facing card (FR-007). The preview reuses ListingCard.tsx
 * so cooks see exactly what buyers will see, including the verification badge.
 */
export function ListingForm({
  cook,
  initialValues,
  onSubmit,
  onCancel,
  submitting = false,
  error = null,
  defaultPickupAddress,
}: ListingFormProps) {
  const [values, setValues] = useState<ListingFormValues>({ ...EMPTY_VALUES, ...initialValues })
  const [clientError, setClientError] = useState<FormError | null>(null)
  const formRef = useRef<HTMLFormElement>(null)
  const shownError = clientError ?? error

  function update<K extends keyof ListingFormValues>(key: K, value: ListingFormValues[K]) {
    setClientError(null)
    setValues((prev) => ({ ...prev, [key]: value }))
  }

  function submit(publish: boolean) {
    if (!formRef.current?.reportValidity()) return
    const problem = validateListing(values)
    setClientError(problem)
    if (problem) return
    onSubmit?.(values, publish)
  }

  const fieldError = (field: string) =>
    shownError?.field === field ? (
      <span role="alert" className="text-sm font-medium text-neutral-900">
        {shownError.message}
      </span>
    ) : null
  const invalid = (field: string) => (shownError?.field === field ? true : undefined)

  const previewListing: ListingItem = useMemo(
    () => ({
      id: 'preview',
      food_description: values.foodDescription || 'Your food description will appear here',
      reduced_price_cents: values.reducedPriceCents,
      portions_remaining: values.portionsTotal,
      pickup_window_start: values.pickupWindowStart || new Date().toISOString(),
      pickup_window_end: values.pickupWindowEnd || new Date().toISOString(),
      photo_url: values.photoUrl || null,
      cook,
    }),
    [values, cook],
  )

  return (
    <div className="grid gap-lg md:grid-cols-2">
      <form
        ref={formRef}
        className="flex flex-col gap-md"
        onSubmit={(e) => {
          e.preventDefault()
          submit(true)
        }}
      >
        <label className="flex flex-col gap-xs text-sm font-medium text-neutral-700">
          Food description
          <input
            type="text"
            value={values.foodDescription}
            onChange={(e) => update('foodDescription', e.target.value)}
            placeholder="e.g. Vegetable lasagne (3 portions)"
            aria-invalid={invalid('food_description')}
            className={INPUT_CLASS}
            required
          />
          {fieldError('food_description')}
        </label>

        <label className="flex flex-col gap-xs text-sm font-medium text-neutral-700">
          Allergens
          <input
            type="text"
            value={values.allergenInfo}
            onChange={(e) => update('allergenInfo', e.target.value)}
            placeholder="e.g. Gluten, dairy (leave empty if none)"
            aria-invalid={invalid('allergen_info')}
            className={INPUT_CLASS}
          />
          {fieldError('allergen_info')}
        </label>

        <div className="grid grid-cols-2 gap-md">
          <label className="flex flex-col gap-xs text-sm font-medium text-neutral-700">
            Reduced price (in cents)
            <input
              type="number"
              min={0}
              value={values.reducedPriceCents}
              onChange={(e) => update('reducedPriceCents', Number(e.target.value))}
              aria-invalid={invalid('reduced_price_cents')}
              className={INPUT_CLASS}
              required
            />
            {fieldError('reduced_price_cents')}
          </label>
          <label className="flex flex-col gap-xs text-sm font-medium text-neutral-700">
            Normal price (in cents)
            <input
              type="number"
              min={1}
              value={values.retailReferencePriceCents}
              onChange={(e) => update('retailReferencePriceCents', Number(e.target.value))}
              aria-invalid={invalid('retail_reference_price_cents')}
              className={INPUT_CLASS}
              required
            />
            {fieldError('retail_reference_price_cents')}
          </label>
        </div>

        <label className="flex flex-col gap-xs text-sm font-medium text-neutral-700">
          Portions available
          <input
            type="number"
            min={1}
            value={values.portionsTotal}
            onChange={(e) => update('portionsTotal', Number(e.target.value))}
            aria-invalid={invalid('total_portions')}
            className={INPUT_CLASS}
            required
          />
          {fieldError('total_portions')}
        </label>

        <div className="grid grid-cols-2 gap-md">
          <label className="flex flex-col gap-xs text-sm font-medium text-neutral-700">
            Pickup start
            <input
              type="datetime-local"
              value={values.pickupWindowStart}
              onChange={(e) => update('pickupWindowStart', e.target.value)}
              aria-invalid={invalid('pickup_window_start')}
              className={INPUT_CLASS}
              required
            />
            {fieldError('pickup_window_start')}
          </label>
          <label className="flex flex-col gap-xs text-sm font-medium text-neutral-700">
            Pickup end
            <input
              type="datetime-local"
              value={values.pickupWindowEnd}
              onChange={(e) => update('pickupWindowEnd', e.target.value)}
              aria-invalid={invalid('pickup_window_end')}
              className={INPUT_CLASS}
              required
            />
            {fieldError('pickup_window_end')}
          </label>
        </div>

        {defaultPickupAddress && (
          <p className="text-sm text-neutral-600">
            Pickup at your default address: <strong>{defaultPickupAddress}</strong>
          </p>
        )}

        <label className="flex flex-col gap-xs text-sm font-medium text-neutral-700">
          Photo URL (optional)
          <input
            type="url"
            value={values.photoUrl}
            onChange={(e) => update('photoUrl', e.target.value)}
            placeholder="https://…"
            aria-invalid={invalid('photo_url')}
            className={INPUT_CLASS}
          />
          {fieldError('photo_url')}
        </label>

        {shownError && !shownError.field && (
          <p role="alert" className="text-sm font-medium text-neutral-900">
            {shownError.message}
          </p>
        )}

        <div className="flex flex-wrap gap-sm">
          <Button type="submit" disabled={submitting}>
            {submitting ? 'Saving…' : 'Publish listing'}
          </Button>
          <Button
            type="button"
            variant="secondary"
            disabled={submitting}
            onClick={() => submit(false)}
          >
            Save as draft
          </Button>
          {onCancel && (
            <Button type="button" variant="secondary" onClick={onCancel}>
              Cancel
            </Button>
          )}
        </div>
      </form>

      <div>
        <p className="mb-xs text-sm font-medium text-neutral-700">Buyer preview</p>
        <ListingCard listing={previewListing} />
      </div>
    </div>
  )
}
