import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { ListingForm, type ListingFormValues } from '@/components/ListingForm'
import type { CookDashboardListingView, VerificationStatus } from '@/types/views'
import type { CookSummary } from '@/services/api'
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
 * preview shows a 0 rating until feature 002 supplies cook ratings.
 */
function toCookSummary(cook: CookProfile): CookSummary {
  return {
    display_name: cook.display_name,
    org_type: cook.org_type,
    verification_status: cook.verification_status,
    rating_avg: 0,
  }
}

// Mock dashboard listings — replace with a real GET /api/v1/cooks/me/listings call.
const MOCK_DASHBOARD_LISTINGS: CookDashboardListingView[] = [
  {
    id: 'dash-001',
    status: 'published',
    foodName: 'Vegetable lasagne (3 portions)',
    portionsTotal: 5,
    portionsClaimed: 2,
    views: 34,
    verificationStatus: 'unverified',
    cardPreview: {
      id: 'dash-001',
      photoUrl: null,
      foodName: 'Vegetable lasagne (3 portions)',
      price: 350,
      portionsRemaining: 3,
      pickupWindow: { start: '2026-08-14T17:00:00Z', end: '2026-08-14T19:00:00Z' },
      pickupLocation: { label: 'Hoofdstraat 12, Amsterdam' },
      cookVerificationStatus: 'unverified',
      status: 'published',
    },
  },
  {
    id: 'dash-002',
    status: 'sold_out',
    foodName: 'Lentil soup with crusty bread',
    portionsTotal: 5,
    portionsClaimed: 5,
    views: 61,
    verificationStatus: 'unverified',
    cardPreview: {
      id: 'dash-002',
      photoUrl: null,
      foodName: 'Lentil soup with crusty bread',
      price: 200,
      portionsRemaining: 0,
      pickupWindow: { start: '2026-08-14T18:00:00Z', end: '2026-08-14T20:00:00Z' },
      pickupLocation: { label: 'Keizersgracht 88, Amsterdam' },
      cookVerificationStatus: 'unverified',
      status: 'sold_out',
    },
  },
  {
    id: 'dash-003',
    status: 'draft',
    foodName: 'Untitled draft listing',
    portionsTotal: 0,
    portionsClaimed: 0,
    views: 0,
    verificationStatus: 'unverified',
    cardPreview: {
      id: 'dash-003',
      photoUrl: null,
      foodName: 'Untitled draft listing',
      price: 0,
      portionsRemaining: 0,
      pickupWindow: { start: '2026-08-14T17:00:00Z', end: '2026-08-14T19:00:00Z' },
      pickupLocation: { label: '' },
      cookVerificationStatus: 'unverified',
      status: 'published',
    },
  },
]

const STATUS_LABEL: Record<CookDashboardListingView['status'], string> = {
  draft: 'draft',
  published: 'published',
  partially_claimed: 'partially claimed',
  sold_out: 'sold out',
  expired: 'expired',
}

// Only "published" and "partially claimed" get the success (light) tone; everything else
// stays neutral — statuses are information, never alarms (Constitution Principle I).
const STATUS_TONE: Record<CookDashboardListingView['status'], 'neutral' | 'success' | 'warning'> =
  {
    draft: 'neutral',
    published: 'success',
    partially_claimed: 'success',
    sold_out: 'neutral',
    expired: 'neutral',
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
      data-testid="registration-card"
      data-status={cook.registration_status}
      aria-labelledby="registration-title"
      className="mb-lg rounded-card border-l-4 border-primary-600 bg-white p-md text-sm text-neutral-700 shadow-xs"
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
      <main className="mx-auto max-w-[48rem] px-md py-lg">
        <Skeleton className="h-48 w-full" label="Loading cook area" />
      </main>
    )
  }

  if (isError || !account) {
    return (
      <main className="mx-auto max-w-[48rem] px-md py-lg">
        <p className="text-neutral-700">We couldn&apos;t load your account. Please refresh the page.</p>
      </main>
    )
  }

  if (!account.email_verified) {
    return (
      <main className="mx-auto max-w-[48rem] px-md py-lg">
        <h1 className="mb-md text-2xl font-bold text-neutral-900">Cook Dashboard</h1>
        <VerifyEmailBanner />
      </main>
    )
  }

  if (!account.cook_profile) return <CookOnboardingPage />

  return <CookDashboard cook={account.cook_profile} />
}

function CookDashboard({ cook }: { cook: CookProfile }) {
  const [isCreating, setIsCreating] = useState(false)
  const cookSummary = toCookSummary(cook)

  function handleSubmit(_values: ListingFormValues) {
    // TODO: wire up to POST /api/v1/listings once the backend is available.
    setIsCreating(false)
  }

  return (
    <main className="mx-auto max-w-[48rem] px-md py-lg font-sans">
      <Link to="/" className="text-sm text-primary-700 hover:underline">
        &larr; Back to discovery
      </Link>

      <h1 className="mb-xs mt-md text-2xl font-bold text-neutral-900">Cook Dashboard</h1>
      <p className="mb-lg text-neutral-600">Manage your surplus-food listings and track claims.</p>

      {/* Verification status notice — always surfaced per Principle I */}
      <VerificationNotice status={cook.verification_status} />

      <RegistrationStatusCard cook={cook} />

      {isCreating ? (
        <section className="mb-2xl">
          <h2 className="mb-md text-lg font-semibold text-neutral-900">Create listing</h2>
          <ListingForm cook={cookSummary} onSubmit={handleSubmit} onCancel={() => setIsCreating(false)} />
        </section>
      ) : (
        <Button type="button" onClick={() => setIsCreating(true)} className="mb-2xl">
          + Create Listing
        </Button>
      )}

      {/* Active listings */}
      <section>
        <h2 className="mb-md text-lg font-semibold text-neutral-900">Your listings</h2>

        {MOCK_DASHBOARD_LISTINGS.length === 0 ? (
          <p className="text-sm text-neutral-500">
            No listings yet. Create your first surplus-food listing above.
          </p>
        ) : (
          <ul className="flex flex-col gap-sm">
            {MOCK_DASHBOARD_LISTINGS.map((listing) => (
              <li
                key={listing.id}
                className="flex flex-wrap items-center justify-between gap-sm rounded-card border border-neutral-200 bg-white p-md shadow-xs"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-sm">
                    <span className="font-semibold text-neutral-900">{listing.foodName}</span>
                    <Badge tone={STATUS_TONE[listing.status]}>{STATUS_LABEL[listing.status]}</Badge>
                  </div>
                  <div className="mt-xs flex flex-wrap gap-md text-sm text-neutral-500">
                    <span>{listing.views} views</span>
                    <span>
                      {listing.portionsClaimed} / {listing.portionsTotal} portions claimed
                    </span>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </main>
  )
}
