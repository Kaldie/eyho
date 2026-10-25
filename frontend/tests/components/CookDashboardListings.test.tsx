/**
 * Cook dashboard against the listings API (T012, T028): real listings, publishing with the
 * contract errors (403 cook_not_registered / email_not_verified, 422 field errors), and the
 * edit-portions / extend / cancel actions with their 409 handling.
 */
import { describe, it, expect, afterEach } from 'vitest'
import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { CookDashboardPage } from '../../src/pages/CookDashboardPage'
import type { CookProfile } from '../../src/services/accounts'
import type { ListingItem } from '../../src/services/api'
import { renderWithAuth, VERIFIED_BUYER } from '../helpers/renderWithAuth'
import { callsTo, stubApi, type Handler } from '../helpers/fakeApi'

const COOK_PROFILE: CookProfile = {
  id: 'cook-1',
  display_name: "Sanne's Kitchen",
  org_type: 'professional',
  phone: '+31612345678',
  default_pickup_address_text: 'Keizersgracht 88, Amsterdam',
  default_pickup_location: { lat: 52.37, lng: 4.89 },
  verification_status: 'unverified',
  kvk_number: '68750110',
  kvk_trade_name: null,
  registration_status: 'active',
  registration_checked_at: null,
  nvwa_declared_at: null,
  can_publish: true,
}
const COOK_ACCOUNT = { ...VERIFIED_BUYER, cook_profile: COOK_PROFILE }

function listing(over: Partial<ListingItem> & { id: string }): ListingItem {
  return {
    food_description: 'Lentil soup',
    reduced_price_cents: 200,
    portions_remaining: 5,
    pickup_window_start: '2099-01-01T17:00:00Z',
    pickup_window_end: '2099-01-01T19:00:00Z',
    distance_km: 0.4,
    photo_url: null,
    cook: {
      display_name: "Sanne's Kitchen",
      org_type: 'professional',
      verification_status: 'unverified',
      rating_avg: 4,
      rating_count: 3,
      verification_cycle_count: 0,
    },
    ...over,
  }
}

const MINE = listing({ id: 'l-mine' })
const OTHERS = listing({
  id: 'l-other',
  food_description: 'Someone else’s curry',
  cook: { ...MINE.cook, display_name: 'Buurthuis Oost' },
})

/** GET /listings answers per `status` query: published has data, draft is empty. */
const LISTINGS_ROUTE: Handler = ({ url }) =>
  new URL(url, 'http://x').searchParams.get('status') === 'published'
    ? { status: 200, body: { listings: [MINE, OTHERS] } }
    : { status: 200, body: { listings: [] } }

afterEach(() => vi.unstubAllGlobals())

function renderDashboard() {
  return renderWithAuth(<CookDashboardPage />, { account: COOK_ACCOUNT })
}

describe('own listings (replacing mocks)', () => {
  it('loads from the API around the cook location and shows only the cook’s own listings', async () => {
    const api = stubApi({ 'GET /api/v1/listings': LISTINGS_ROUTE })
    renderDashboard()

    const rows = await screen.findAllByTestId('cook-listing')
    expect(rows).toHaveLength(1)
    expect(within(rows[0]).getByText('Lentil soup')).toBeInTheDocument()
    expect(within(rows[0]).getByTestId('listing-verification')).toHaveTextContent('unverified')
    expect(screen.queryByText(/Someone else/)).not.toBeInTheDocument()

    const url = new URL(String(callsTo(api, 'GET', '/api/v1/listings')[0][0]), 'http://x')
    expect(url.searchParams.get('lat')).toBe('52.37')
    expect(url.searchParams.get('lng')).toBe('4.89')
  })

  it('shows an empty state when the cook has none', async () => {
    stubApi({ 'GET /api/v1/listings': { status: 200, body: { listings: [] } } })
    renderDashboard()
    expect(await screen.findByText(/No listings yet/i)).toBeInTheDocument()
  })
})

describe('publishing', () => {
  async function fillForm(user: ReturnType<typeof userEvent.setup>) {
    await user.click(await screen.findByRole('button', { name: /Create Listing/i }))
    await user.type(screen.getByRole('textbox', { name: /food description/i }), 'Pasta')
    fireEvent.change(screen.getByLabelText(/^reduced price/i), { target: { value: '300' } })
    fireEvent.change(screen.getByLabelText(/^normal price/i), { target: { value: '900' } })
    fireEvent.change(screen.getByLabelText(/portions available/i), { target: { value: '4' } })
    fireEvent.change(screen.getByLabelText(/pickup start/i), { target: { value: '2099-01-01T17:00' } })
    fireEvent.change(screen.getByLabelText(/pickup end/i), { target: { value: '2099-01-01T19:00' } })
  }

  it('POSTs the contract body without cook_id or pickup overrides, then confirms with the unverified flag', async () => {
    const created = listing({ id: 'new-1', food_description: 'Pasta', status: 'published' })
    const api = stubApi({
      'GET /api/v1/listings': { status: 200, body: { listings: [] } },
      'POST /api/v1/listings': { status: 201, body: created },
    })
    renderDashboard()
    const user = userEvent.setup()
    await fillForm(user)
    await user.click(screen.getByRole('button', { name: 'Publish listing' }))

    expect(await screen.findByTestId('dashboard-notice')).toHaveTextContent(/labelled "unverified"/)
    const body = JSON.parse(String((callsTo(api, 'POST', '/api/v1/listings')[0][1] as RequestInit).body))
    expect(body).toMatchObject({
      food_description: 'Pasta',
      allergen_info: '',
      total_portions: 4,
      reduced_price_cents: 300,
      retail_reference_price_cents: 900,
      publish: true,
    })
    expect(body.pickup_window_end).toBe(new Date('2099-01-01T19:00').toISOString())
    expect(body).not.toHaveProperty('cook_id')
    expect(body).not.toHaveProperty('pickup_address_text')
    expect(body).not.toHaveProperty('photo_url')
    expect(await screen.findAllByTestId('cook-listing')).toHaveLength(1)
  })

  it('saves a draft with publish false', async () => {
    const api = stubApi({
      'GET /api/v1/listings': { status: 200, body: { listings: [] } },
      'POST /api/v1/listings': { status: 201, body: listing({ id: 'd-1', status: 'draft' }) },
    })
    renderDashboard()
    const user = userEvent.setup()
    await fillForm(user)
    await user.click(screen.getByRole('button', { name: 'Save as draft' }))
    expect(await screen.findByTestId('dashboard-notice')).toHaveTextContent(/Draft saved/)
    const body = JSON.parse(String((callsTo(api, 'POST', '/api/v1/listings')[0][1] as RequestInit).body))
    expect(body.publish).toBe(false)
  })

  it('blocks a reduced price at or above the normal price before calling the API', async () => {
    const api = stubApi({ 'GET /api/v1/listings': { status: 200, body: { listings: [] } } })
    renderDashboard()
    const user = userEvent.setup()
    await fillForm(user)
    fireEvent.change(screen.getByLabelText(/^reduced price/i), { target: { value: '900' } })
    await user.click(screen.getByRole('button', { name: 'Publish listing' }))
    expect(await screen.findByText(/must be lower than the normal price/i)).toBeInTheDocument()
    expect(callsTo(api, 'POST', '/api/v1/listings')).toHaveLength(0)
  })

  it('403 cook_not_registered points to the registration card and keeps the form for a draft', async () => {
    stubApi({
      'GET /api/v1/listings': { status: 200, body: { listings: [] } },
      'POST /api/v1/listings': {
        status: 403,
        body: { error: 'cook_not_registered', registration_status: 'pending' },
      },
    })
    renderDashboard()
    const user = userEvent.setup()
    await fillForm(user)
    await user.click(screen.getByRole('button', { name: 'Publish listing' }))

    const alert = await screen.findByTestId('publish-not-registered')
    expect(alert).toHaveTextContent(/pending/)
    expect(within(alert).getByRole('link', { name: /business registration card/i })).toHaveAttribute(
      'href',
      '#registration',
    )
    expect(screen.getByTestId('registration-card')).toHaveAttribute('id', 'registration')
    expect(screen.getByRole('button', { name: 'Save as draft' })).toBeInTheDocument()
  })

  it('403 email_not_verified shows the verify banner', async () => {
    stubApi({
      'GET /api/v1/listings': { status: 200, body: { listings: [] } },
      'POST /api/v1/listings': { status: 403, body: { error: 'email_not_verified' } },
    })
    renderDashboard()
    const user = userEvent.setup()
    await fillForm(user)
    await user.click(screen.getByRole('button', { name: 'Publish listing' }))
    expect(await screen.findByTestId('verify-email-banner')).toBeInTheDocument()
  })

  it('422 {error, field} is shown next to the named field', async () => {
    stubApi({
      'GET /api/v1/listings': { status: 200, body: { listings: [] } },
      'POST /api/v1/listings': {
        status: 422,
        body: { error: 'pickup_window_in_past', field: 'pickup_window_end' },
      },
    })
    renderDashboard()
    const user = userEvent.setup()
    await fillForm(user)
    await user.click(screen.getByRole('button', { name: 'Publish listing' }))
    const field = await screen.findByLabelText(/pickup end/i)
    await waitFor(() => expect(field).toHaveAttribute('aria-invalid', 'true'))
    expect(screen.getByText('Pickup window in past.')).toBeInTheDocument()
  })
})

describe('listing actions', () => {
  function stubWithPatch(patch: Handler) {
    return stubApi({
      'GET /api/v1/listings': LISTINGS_ROUTE,
      'PATCH /api/v1/listings/l-mine': patch,
    })
  }
  const patchBody = (api: ReturnType<typeof stubApi>) =>
    JSON.parse(String((callsTo(api, 'PATCH', '/api/v1/listings/l-mine')[0][1] as RequestInit).body))

  it('edits portions via PATCH portions_remaining and shows the new count', async () => {
    const api = stubWithPatch({ status: 200, body: { ...MINE, portions_remaining: 3, status: 'published' } })
    renderDashboard()
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Edit portions' }))
    const input = screen.getByLabelText('Portions remaining')
    await user.clear(input)
    await user.type(input, '3')
    await user.click(screen.getByRole('button', { name: 'Save portions' }))

    expect(await screen.findByText('3 portions left')).toBeInTheDocument()
    expect(patchBody(api)).toEqual({ portions_remaining: 3 })
  })

  it('409 when going below the claimed portions explains it and keeps the editor open', async () => {
    stubWithPatch({ status: 409, body: { error: 'below_claimed' } })
    renderDashboard()
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Edit portions' }))
    const input = screen.getByLabelText('Portions remaining')
    await user.clear(input)
    await user.type(input, '0')
    await user.click(screen.getByRole('button', { name: 'Save portions' }))

    expect(await screen.findByTestId('listing-action-error')).toHaveTextContent(/already claimed/i)
    expect(screen.getByLabelText('Portions remaining')).toBeInTheDocument()
    expect(screen.getByText('5 portions left')).toBeInTheDocument()
  })

  it('extends the pickup window via PATCH pickup_window_end, rejecting an earlier time client-side', async () => {
    const api = stubWithPatch({
      status: 200,
      body: { ...MINE, pickup_window_end: '2099-01-01T21:00:00Z', status: 'published' },
    })
    renderDashboard()
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Extend pickup' }))
    const input = screen.getByLabelText('New pickup end')

    fireEvent.change(input, { target: { value: '2000-01-01T10:00' } })
    await user.click(screen.getByRole('button', { name: 'Extend pickup' }))
    expect(await screen.findByTestId('listing-action-error')).toHaveTextContent(/later than/i)
    expect(callsTo(api, 'PATCH', '/api/v1/listings/l-mine')).toHaveLength(0)

    fireEvent.change(input, { target: { value: '2099-01-02T21:00' } })
    await user.click(screen.getByRole('button', { name: 'Extend pickup' }))
    await waitFor(() => expect(callsTo(api, 'PATCH', '/api/v1/listings/l-mine')).toHaveLength(1))
    expect(patchBody(api)).toEqual({ pickup_window_end: new Date('2099-01-02T21:00').toISOString() })
  })

  it('asks for confirmation before cancelling, then PATCHes status cancelled', async () => {
    const api = stubWithPatch({ status: 200, body: { ...MINE, status: 'cancelled' } })
    renderDashboard()
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Cancel listing' }))

    const confirm = screen.getByRole('group', { name: /confirm cancelling/i })
    expect(confirm).toHaveTextContent(/refunded automatically/i)
    expect(callsTo(api, 'PATCH', '/api/v1/listings/l-mine')).toHaveLength(0)

    await user.click(within(confirm).getByRole('button', { name: 'Yes, cancel listing' }))
    await waitFor(() => expect(screen.getByTestId('cook-listing')).toHaveAttribute('data-status', 'cancelled'))
    expect(patchBody(api)).toEqual({ status: 'cancelled' })
    expect(screen.queryByRole('button', { name: 'Edit portions' })).not.toBeInTheDocument()
  })

  it('keeping the listing makes no request', async () => {
    const api = stubWithPatch({ status: 200, body: MINE })
    renderDashboard()
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Cancel listing' }))
    await user.click(screen.getByRole('button', { name: 'Keep listing' }))
    expect(callsTo(api, 'PATCH', '/api/v1/listings/l-mine')).toHaveLength(0)
    expect(screen.getByRole('button', { name: 'Edit portions' })).toBeInTheDocument()
  })
})
