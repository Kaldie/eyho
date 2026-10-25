import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { ListingForm, type FormError, type ListingFormValues } from '@/components/ListingForm'
import { CookListingRow } from '@/components/CookListingRow'
import type { VerificationStatus } from '@/types/views'
import {
  ApiError,
  COOK_LISTINGS_QUERY_KEY,
  createListing,
  useCookListings,
  validationMessage,
} from '@/services/api'
import type { CookSummary, CreateListingInput, ListingItem } from '@/services/api'
import type { CookProfile, RegistrationStatus } from '@/services/accounts'
import { useAccount } from '@/auth/useAccount'
import { Skeleton } from '@/components/ui/Skeleton'
import { VerifyEmailBanner } from '@/components/VerifyEmailBanner'
import { CookOnboardingPage } from '@/pages/CookOnboardingPage'

/**
 * CookDashboardPage — the cook's management view.
 *
 * Surfaces the cook's listings with status/stats (FR-006), a neutral verification-state
 * indicator (Acceptance Scenario 3 / Edge Cases), and a creation/edit form with a
 * live-updating buyer-facing preview (FR-007).
 */

/**
 * The contract's cook_profile (GET /me) carries no rating fields, so the listing-form
 * preview shows "No reviews yet"; real listings returned by the API carry the true values.
 */
function toCookSummary(cook: CookProfile): CookSummary {
  return {
    display_name: cook.display_name,
    org_type: cook.org_type,
    verification_status: cook.verification_status,
    rating_avg: 0,
    rating_count: 0,
    verification_cycle_count: 0,
  }
}

interface VerificationNoticeProps {
  status: VerificationStatus
}

/**
 * Neutral, non-alarming verification-state indicator (T038).
 * Deliberately avoids red/warning styling regardless of status — an "unverified" cook
 * is informational, not an error state, per Principle I and the spec's Edge Cases.
 */
function VerificationNotice({ status }: VerificationNoticeProps) {
  const isVerified = status === 'verified'
  return (
    <div
      className="mb-lg flex flex-col gap-xs rounded-card border border-neutral-200 bg-neutral-50 p-md text-sm text-neutral-700"
      data-testid="verification-notice"
    >
      <div className="flex items-center gap-sm">
        <Badge tone={isVerified ? 'success' : 'neutral'}>
          {isVerified ? 'verified' : 'unverified'}
        </Badge>
        <span className="font-semibold text-neutral-900">
          {isVerified ? 'You are a verified cook' : 'Your account is not yet verified'}
        </span>
      </div>
      <p className="text-neutral-600">
        {isVerified
          ? 'Buyers see a "verified" badge on your listings. Keep up great ratings to maintain it.'
          : 'Buyers will see an "unverified" badge on your listings — this is shown for transparency, not as a warning. Earn verified status by receiving 5 ratings of 4 stars or higher.'}
      </p>
    </div>
  )
}

const REGISTRATION_COPY: Record<
  RegistrationStatus,
  { title: string; body: (cook: CookProfile) => string }
> = {
  active: {
    title: 'Registered',
    body: (cook) =>
      `Your KvK registration${cook.kvk_trade_name ? ` as ${cook.kvk_trade_name}` : ''} is confirmed (KvK ${cook.kvk_number}). You can publish listings.`,
  },
  pending: {
    title: 'Checking your registration',
    body: () =>
      'We couldn’t reach the business register just now and are retrying in the background. You can publish as soon as the check succeeds.',
  },
  inactive: {
    title: 'Registration inactive',
    body: (cook) =>
      `KVK shows registration ${cook.kvk_number} as no longer active, so new listings are paused. Claims you already have are not affected. Update your registration with KVK; we re-check regularly and publishing resumes once it is active.`,
  },
}

/**
 * Registration status card (FR-022). Deliberately separate from the verification notice
 * and its badge: registration is a legal check on the business, "verified" is earned from
 * buyer ratings (Principle I), and buyers never see this card.
 */
export function RegistrationStatusCard({ cook }: { cook: CookProfile }) {
  const copy = REGISTRATION_COPY[cook.registration_status]
  return (
    <section
      id="registration"
      data-testid="registration-card"
      data-status={cook.registration_status}
      aria-labelledby="registration-title"
      className="mb-lg rounded-card border-l-4 border-primary-600 bg-white p-md text-sm text-neutral-700 shadow-sm"
    >
      <p className="mb-xs text-xs font-semibold uppercase tracking-wide text-neutral-500">
        Business registration
      </p>
      <h2 id="registration-title" className="mb-xs text-base font-bold text-neutral-900">
        {copy.title}
      </h2>
      <p>{copy.body(cook)}</p>
    </section>
  )
}

/**
 * Cook area gate: sign-in is handled by RequireSignIn in the router. Here an unverified
 * email shows the verify banner instead of proceeding (FR-006), and an account without a
 * cook profile gets onboarding (FR-009).
 */
export function CookDashboardPage() {
  const { account, isLoading, isError } = useAccount()

  if (isLoading) {
    return (
      <main className="mx-auto max-w-3xl px-md py-lg">
        <Skeleton className="h-48 w-full" label="Loading cook area" />
      </main>
    )
  }

  if (isError || !account) {
    return (
      <main className="mx-auto max-w-3xl px-md py-lg">
        <p className="text-neutral-700">We couldn&apos;t load your account. Please refresh the page.</p>
      </main>
    )
  }

  if (!account.email_verified) {
    return (
      <main className="mx-auto max-w-3xl px-md py-lg">
        <h1 className="mb-md text-2xl font-bold text-neutral-900">Cook Dashboard</h1>
        <VerifyEmailBanner />
      </main>
    )
  }

  if (!account.cook_profile) return <CookOnboardingPage />

  return <CookDashboard cook={account.cook_profile} />
}

function toCreateInput(values: ListingFormValues, publish: boolean): CreateListingInput {
  return {
    food_description: values.foodDescription.trim(),
    allergen_info: values.allergenInfo.trim(),
    ...(values.photoUrl.trim() ? { photo_url: values.photoUrl.trim() } : {}),
    total_portions: values.portionsTotal,
    reduced_price_cents: values.reducedPriceCents,
    retail_reference_price_cents: values.retailReferencePriceCents,
    // pickup_location / pickup_address_text omitted: the API uses the cook's default address.
    pickup_window_start: new Date(values.pickupWindowStart).toISOString(),
    pickup_window_end: new Date(values.pickupWindowEnd).toISOString(),
    publish,
  }
}

/** What stopped a create, beyond field validation. */
type PublishProblem =
  | { kind: 'cook_not_registered'; registrationStatus: string | null }
  | { kind: 'email_not_verified' }
  | { kind: 'other'; message: string }

function CookDashboard({ cook }: { cook: CookProfile }) {
  const [isCreating, setIsCreating] = useState(false)
  const [formError, setFormError] = useState<FormError | null>(null)
  const [problem, setProblem] = useState<PublishProblem | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  /** Listings changed in this session (created, edited, cancelled), keyed by id. */
  const [overrides, setOverrides] = useState<Record<string, ListingItem>>({})
  const queryClient = useQueryClient()
  const cookSummary = toCookSummary(cook)
  const listings = useCookListings(cook)

  const create = useMutation({
    mutationFn: ({ values, publish }: { values: ListingFormValues; publish: boolean }) =>
      createListing(toCreateInput(values, publish)),
    onSuccess: (listing) => {
      setOverrides((o) => ({ ...o, [listing.id]: listing }))
      setIsCreating(false)
      setNotice(
        listing.status === 'draft'
          ? 'Draft saved. Buyers can’t see it until it is published.'
          : listing.cook.verification_status === 'unverified'
            ? 'Listing published. Buyers see it labelled "unverified" until you earn verified status.'
            : 'Listing published.',
      )
      void queryClient.invalidateQueries({ queryKey: COOK_LISTINGS_QUERY_KEY })
    },
    onError: (err) => {
      if (err instanceof ApiError && err.status === 403 && err.code === 'cook_not_registered') {
        setProblem({
          kind: 'cook_not_registered',
          registrationStatus:
            typeof err.body.registration_status === 'string' ? err.body.registration_status : null,
        })
      } else if (err instanceof ApiError && err.status === 403 && err.code === 'email_not_verified') {
        setProblem({ kind: 'email_not_verified' })
      } else if (err instanceof ApiError && err.status === 422) {
        setFormError({
          message: validationMessage(err),
          field: typeof err.body.field === 'string' ? err.body.field : undefined,
        })
      } else if (err instanceof ApiError && err.code === 'cook_profile_required') {
        setProblem({ kind: 'other', message: 'Set up your cook profile before creating listings.' })
      } else {
        setProblem({ kind: 'other', message: "We couldn't save your listing. Please try again." })
      }
    },
  })

  function handleSubmit(values: ListingFormValues, publish: boolean) {
    setFormError(null)
    setProblem(null)
    setNotice(null)
    create.mutate({ values, publish })
  }

  function handleUpdated(listing: ListingItem) {
    setOverrides((o) => ({ ...o, [listing.id]: listing }))
    void queryClient.invalidateQueries({ queryKey: COOK_LISTINGS_QUERY_KEY })
  }

  const fetched = listings.data ?? []
  const fetchedIds = new Set(fetched.map((l) => l.id))
  const rows = [
    ...fetched.map((l) => overrides[l.id] ?? l),
    ...Object.values(overrides).filter((l) => !fetchedIds.has(l.id)),
  ]

  return (
    <main className="mx-auto max-w-3xl px-md py-lg font-sans">
      <Link to="/" className="text-sm text-primary-700 hover:underline">
        &larr; Back to discovery
      </Link>

      <h1 className="mb-xs mt-md text-2xl font-bold text-neutral-900">Cook Dashboard</h1>
      <p className="mb-lg text-neutral-600">Manage your surplus-food listings and track claims.</p>

      {/* Verification status notice — always surfaced per Principle I */}
      <VerificationNotice status={cook.verification_status} />

      <RegistrationStatusCard cook={cook} />

      {notice && (
        <div
          role="status"
          data-testid="dashboard-notice"
          className="mb-lg rounded-card border border-primary-200 bg-primary-50 px-lg py-sm text-primary-800"
        >
          {notice}
        </div>
      )}

      {problem?.kind === 'cook_not_registered' && (
        <div
          role="alert"
          data-testid="publish-not-registered"
          className="mb-lg rounded-card border border-neutral-300 bg-neutral-50 p-md text-sm text-neutral-800"
        >
          <p className="mb-xs font-semibold">Publishing is paused until your registration is active</p>
          <p>
            Your business registration is {problem.registrationStatus ?? 'not confirmed'}, so this
            listing was not published. See the{' '}
            <a href="#registration" className="font-semibold text-primary-700 underline">
              business registration card
            </a>{' '}
            above for what to do. You can still save drafts.
          </p>
        </div>
      )}

      {problem?.kind === 'email_not_verified' && <VerifyEmailBanner force className="mb-lg" />}

      {problem?.kind === 'other' && (
        <p role="alert" className="mb-lg text-sm font-medium text-neutral-900">
          {problem.message}
        </p>
      )}

      {isCreating ? (
        <section className="mb-2xl">
          <h2 className="mb-md text-lg font-semibold text-neutral-900">Create listing</h2>
          <ListingForm
            cook={cookSummary}
            defaultPickupAddress={cook.default_pickup_address_text}
            submitting={create.isPending}
            error={formError}
            onSubmit={handleSubmit}
            onCancel={() => {
              setIsCreating(false)
              setFormError(null)
              setProblem(null)
            }}
          />
        </section>
      ) : (
        <Button
          type="button"
          onClick={() => {
            setNotice(null)
            setIsCreating(true)
          }}
          className="mb-2xl"
        >
          + Create Listing
        </Button>
      )}

      {/* Active listings */}
      <section>
        <h2 className="mb-md text-lg font-semibold text-neutral-900">Your listings</h2>

        {listings.isLoading && <Skeleton className="h-24 w-full" label="Loading your listings" />}

        {listings.isError && (
          <p role="alert" className="text-sm text-neutral-700">
            We couldn&apos;t load your listings. Refresh the page to try again.
          </p>
        )}

        {!listings.isLoading && rows.length === 0 && !listings.isError && (
          <p className="text-sm text-neutral-500">
            No listings yet. Create your first surplus-food listing above.
          </p>
        )}

        {rows.length > 0 && (
          <ul className="flex flex-col gap-sm">
            {rows.map((listing) => (
              <CookListingRow
                key={listing.id}
                listing={listing}
                onUpdated={handleUpdated}
                onEmailNotVerified={() => setProblem({ kind: 'email_not_verified' })}
              />
            ))}
          </ul>
        )}
      </section>
    </main>
  )
}
