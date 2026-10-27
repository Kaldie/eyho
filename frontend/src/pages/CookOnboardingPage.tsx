import { useState, type FormEvent } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/services/api'
import type { OrgType } from '@/services/api'
import { ME_QUERY_KEY, createCookProfile } from '@/services/accounts'
import { AddressSearch, type AddressSelection } from '@/components/AddressSearch'
import { Button } from '@/components/ui/Button'
import { TextField } from '@/components/ui/TextField'

/** Version of the declaration wording below; stored with the declaration time (FR-019). */
export const NVWA_DECLARATION_VERSION = '2026-10-25'
export const NVWA_DECLARATION_TEXT =
  'I declare that my kitchen is registered with the NVWA and that I work with a hygiene code.'

export interface OnboardingValues {
  displayName: string
  orgType: OrgType | ''
  phone: string
  address: AddressSelection | null
  kvkNumber: string
  declared: boolean
}

export type OnboardingField =
  | 'display_name'
  | 'org_type'
  | 'phone'
  | 'address'
  | 'kvk_number'
  | 'declaration'

export type OnboardingErrors = Partial<Record<OnboardingField, string>>

/** Client-side validation with one clear message per field (FR-009). */
export function validateOnboarding(v: OnboardingValues): OnboardingErrors {
  const errors: OnboardingErrors = {}
  if (!v.displayName.trim()) errors.display_name = 'Enter the name buyers will see.'
  if (!v.orgType) errors.org_type = 'Choose non-profit or professional.'
  if (!v.phone.trim()) errors.phone = 'Enter a phone number for pickup coordination.'
  else if (v.phone.replace(/\D/g, '').length < 8) errors.phone = 'Check your phone number.'
  if (!v.address) errors.address = 'Search for your address and choose one of the suggestions.'
  const kvk = v.kvkNumber.trim()
  if (!kvk) errors.kvk_number = 'Enter your 8-digit KvK number.'
  else if (!/^\d{8}$/.test(kvk)) errors.kvk_number = 'A KvK number has exactly 8 digits.'
  if (!v.declared) errors.declaration = 'Confirm the declaration to continue.'
  return errors
}

const FIELD_FROM_API: Record<string, OnboardingField> = {
  display_name: 'display_name',
  org_type: 'org_type',
  phone: 'phone',
  default_pickup_address_text: 'address',
  default_pickup_location: 'address',
  kvk_number: 'kvk_number',
}

const FIELD_LABEL: Record<OnboardingField, string> = {
  display_name: 'display name',
  org_type: 'organisation type',
  phone: 'phone number',
  address: 'pickup address',
  kvk_number: 'KvK number',
  declaration: 'declaration',
}

export interface OnboardingFailure {
  /** Shown under the matching field, if the error belongs to one. */
  field?: OnboardingField
  message: string
}

/** Maps contract errors (422/409 bodies) to messages a cook can act on. */
export function describeOnboardingError(err: unknown): OnboardingFailure {
  if (err instanceof ApiError) {
    const apiField = typeof err.body.field === 'string' ? err.body.field : ''
    const field = FIELD_FROM_API[apiField]
    switch (err.code) {
      case 'kvk_not_found':
        return {
          field: 'kvk_number',
          message:
            'We couldn’t find this KvK number in the Handelsregister. Check the number and try again.',
        }
      case 'kvk_inactive':
        return {
          field: 'kvk_number',
          message:
            'This KvK registration is no longer active, so you can’t sell on eyho with it. Update your registration at KVK, or use another number.',
        }
      case 'declaration_required':
        return { field: 'declaration', message: 'Confirm the declaration to continue.' }
      case 'missing_field':
        return {
          field,
          message: `Please fill in your ${field ? FIELD_LABEL[field] : 'details'}.`,
        }
      case 'invalid_field':
        return {
          field,
          message: `Check your ${field ? FIELD_LABEL[field] : 'details'}; it doesn’t look right.`,
        }
      case 'cook_profile_exists':
        return { message: 'You already have a cook profile.' }
      case 'email_not_verified':
        return { message: 'Verify your email before becoming a cook.' }
    }
  }
  return { message: 'We couldn’t save your cook profile. Check your connection and try again.' }
}

/**
 * CookOnboardingPage (T026) — turns a buyer account into a cook. New cooks start
 * "unverified" (FR-010) and the form says so. The KvK number is checked by the backend;
 * not-found/inactive results are shown inline and nothing is saved.
 */
export function CookOnboardingPage() {
  const queryClient = useQueryClient()
  const [values, setValues] = useState<OnboardingValues>({
    displayName: '',
    orgType: '',
    phone: '',
    address: null,
    kvkNumber: '',
    declared: false,
  })
  const [errors, setErrors] = useState<OnboardingErrors>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [pendingNotice, setPendingNotice] = useState(false)

  function patch(next: Partial<OnboardingValues>, clear?: OnboardingField) {
    setValues((v) => ({ ...v, ...next }))
    if (clear) setErrors((e) => ({ ...e, [clear]: undefined }))
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setFormError(null)
    const found = validateOnboarding(values)
    setErrors(found)
    if (Object.keys(found).length > 0) return

    setSubmitting(true)
    try {
      const result = await createCookProfile({
        display_name: values.displayName.trim(),
        org_type: values.orgType as OrgType,
        phone: values.phone.trim(),
        default_pickup_address_text: values.address!.text,
        default_pickup_location: { lat: values.address!.lat, lng: values.address!.lng },
        kvk_number: values.kvkNumber.trim(),
        nvwa_declaration: { accepted: true, version: NVWA_DECLARATION_VERSION },
      })
      if (result.status === 202) setPendingNotice(true)
      await queryClient.invalidateQueries({ queryKey: ME_QUERY_KEY })
    } catch (err) {
      if (err instanceof ApiError && err.code === 'cook_profile_exists') {
        await queryClient.invalidateQueries({ queryKey: ME_QUERY_KEY })
        return
      }
      const failure = describeOnboardingError(err)
      if (failure.field) setErrors({ [failure.field]: failure.message })
      else setFormError(failure.message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <main className="mx-auto max-w-[36rem] px-md py-lg sm:px-lg">
      <h1 className="mb-xs text-2xl font-bold text-neutral-900">Become a cook</h1>
      <p className="mb-lg text-neutral-600">
        Sell your surplus cooked food at a reduced price to people nearby. You start as{' '}
        <strong>unverified</strong>; buyers see that label until you have earned 5 ratings of 4
        stars or more.
      </p>

      {pendingNotice && (
        <p role="status" className="mb-md rounded-card border border-neutral-300 bg-neutral-50 px-md py-sm text-sm text-neutral-800">
          Your profile is saved. We couldn&apos;t reach the business register just now, so we&apos;re
          checking your registration in the background.
        </p>
      )}

      <form onSubmit={(e) => void handleSubmit(e)} noValidate className="flex flex-col gap-md">
        <TextField
          label="Display name"
          value={values.displayName}
          error={errors.display_name}
          onChange={(e) => patch({ displayName: e.target.value }, 'display_name')}
          autoComplete="organization"
        />

        <fieldset aria-describedby={errors.org_type ? 'org-type-error' : undefined}>
          <legend className="mb-xs text-sm font-semibold text-neutral-800">Organisation type</legend>
          <div className="flex flex-wrap gap-md">
            {(
              [
                ['nonprofit', 'Non-profit'],
                ['professional', 'Professional'],
              ] as const
            ).map(([value, label]) => (
              <label key={value} className="flex min-h-11 items-center gap-sm text-neutral-800">
                <input
                  type="radio"
                  name="org_type"
                  value={value}
                  checked={values.orgType === value}
                  onChange={() => patch({ orgType: value }, 'org_type')}
                />
                {label}
              </label>
            ))}
          </div>
          {errors.org_type && (
            <p id="org-type-error" role="alert" className="mt-xs text-sm font-medium text-danger">
              {errors.org_type}
            </p>
          )}
        </fieldset>

        <TextField
          label="Phone number"
          type="tel"
          value={values.phone}
          error={errors.phone}
          hint="Used to coordinate pickups. We don't verify it."
          onChange={(e) => patch({ phone: e.target.value }, 'phone')}
          autoComplete="tel"
        />

        <AddressSearch
          label="Default pickup address"
          value={values.address}
          error={errors.address}
          onChange={(address) => patch({ address }, 'address')}
        />

        <TextField
          label="KvK number"
          inputMode="numeric"
          value={values.kvkNumber}
          error={errors.kvk_number}
          hint="8 digits. We check it against the Dutch business register."
          onChange={(e) => patch({ kvkNumber: e.target.value }, 'kvk_number')}
        />

        <div>
          <label className="flex items-start gap-sm text-sm text-neutral-800">
            <input
              type="checkbox"
              className="mt-[0.2rem]"
              checked={values.declared}
              onChange={(e) => patch({ declared: e.target.checked }, 'declaration')}
            />
            <span>{NVWA_DECLARATION_TEXT}</span>
          </label>
          {errors.declaration && (
            <p role="alert" className="mt-xs text-sm font-medium text-danger">
              {errors.declaration}
            </p>
          )}
        </div>

        {formError && (
          <p role="alert" className="rounded-card border border-neutral-300 bg-neutral-50 px-md py-sm text-sm text-neutral-800">
            {formError}
          </p>
        )}

        <Button type="submit" disabled={submitting}>
          {submitting ? 'Saving…' : 'Become a cook'}
        </Button>
      </form>
    </main>
  )
}
