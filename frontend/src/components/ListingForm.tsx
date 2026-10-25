import { useMemo, useState } from 'react'
import { Button } from '@/components/ui/Button'
import { ListingCard } from '@/components/ListingCard'
import type { ListingItem, CookSummary } from '@/services/api'

export interface ListingFormValues {
  foodDescription: string
  reducedPriceCents: number
  portionsTotal: number
  pickupWindowStart: string
  pickupWindowEnd: string
  photoUrl: string
}

export interface ListingFormProps {
  /** The signed-in cook's own profile — used to build the live preview card. */
  cook: CookSummary
  initialValues?: Partial<ListingFormValues>
  onSubmit?: (values: ListingFormValues) => void
  onCancel?: () => void
}

const EMPTY_VALUES: ListingFormValues = {
  foodDescription: '',
  reducedPriceCents: 0,
  portionsTotal: 1,
  pickupWindowStart: '',
  pickupWindowEnd: '',
  photoUrl: '',
}

/**
 * ListingForm — creation/edit form for a cook's listing, with a live-updating
 * preview of the buyer-facing card (FR-007). The preview reuses ListingCard.tsx
 * so cooks see exactly what buyers will see, including the verification badge.
 */
export function ListingForm({ cook, initialValues, onSubmit, onCancel }: ListingFormProps) {
  const [values, setValues] = useState<ListingFormValues>({ ...EMPTY_VALUES, ...initialValues })

  function update<K extends keyof ListingFormValues>(key: K, value: ListingFormValues[K]) {
    setValues((prev) => ({ ...prev, [key]: value }))
  }

  const previewListing: ListingItem = useMemo(
    () => ({
      id: 'preview',
      food_description: values.foodDescription || 'Your food description will appear here',
      reduced_price_cents: values.reducedPriceCents,
      portions_remaining: values.portionsTotal,
      pickup_window_start: values.pickupWindowStart || new Date().toISOString(),
      pickup_window_end: values.pickupWindowEnd || new Date().toISOString(),
      distance_km: 0,
      photo_url: values.photoUrl || null,
      cook,
    }),
    [values, cook],
  )

  return (
    <div className="grid gap-lg md:grid-cols-2">
      <form
        className="flex flex-col gap-md"
        onSubmit={(e) => {
          e.preventDefault()
          onSubmit?.(values)
        }}
      >
        <label className="flex flex-col gap-xs text-sm font-medium text-neutral-700">
          Food description
          <input
            type="text"
            value={values.foodDescription}
            onChange={(e) => update('foodDescription', e.target.value)}
            placeholder="e.g. Vegetable lasagne (3 portions)"
            className="rounded-card border border-neutral-300 px-sm py-xs text-base text-neutral-900"
            required
          />
        </label>

        <label className="flex flex-col gap-xs text-sm font-medium text-neutral-700">
          Reduced price (in cents)
          <input
            type="number"
            min={0}
            value={values.reducedPriceCents}
            onChange={(e) => update('reducedPriceCents', Number(e.target.value))}
            className="rounded-card border border-neutral-300 px-sm py-xs text-base text-neutral-900"
            required
          />
        </label>

        <label className="flex flex-col gap-xs text-sm font-medium text-neutral-700">
          Portions available
          <input
            type="number"
            min={1}
            value={values.portionsTotal}
            onChange={(e) => update('portionsTotal', Number(e.target.value))}
            className="rounded-card border border-neutral-300 px-sm py-xs text-base text-neutral-900"
            required
          />
        </label>

        <div className="grid grid-cols-2 gap-md">
          <label className="flex flex-col gap-xs text-sm font-medium text-neutral-700">
            Pickup start
            <input
              type="datetime-local"
              value={values.pickupWindowStart}
              onChange={(e) => update('pickupWindowStart', e.target.value)}
              className="rounded-card border border-neutral-300 px-sm py-xs text-base text-neutral-900"
              required
            />
          </label>
          <label className="flex flex-col gap-xs text-sm font-medium text-neutral-700">
            Pickup end
            <input
              type="datetime-local"
              value={values.pickupWindowEnd}
              onChange={(e) => update('pickupWindowEnd', e.target.value)}
              className="rounded-card border border-neutral-300 px-sm py-xs text-base text-neutral-900"
              required
            />
          </label>
        </div>

        <label className="flex flex-col gap-xs text-sm font-medium text-neutral-700">
          Photo URL (optional)
          <input
            type="url"
            value={values.photoUrl}
            onChange={(e) => update('photoUrl', e.target.value)}
            placeholder="https://…"
            className="rounded-card border border-neutral-300 px-sm py-xs text-base text-neutral-900"
          />
        </label>

        <div className="flex gap-sm">
          <Button type="submit">Save listing</Button>
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
