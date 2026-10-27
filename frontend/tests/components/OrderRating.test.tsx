/**
 * Order history against the claims API shape, and the rating prompt on collected orders
 * (T025, FR-013): POST /claims/{id}/rating with 409 already_rated and 403 handling.
 */
import { describe, it, expect, afterEach } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import { vi } from 'vitest'
import { OrderHistoryPage } from '../../src/pages/OrderHistoryPage'
import type { ClaimItem } from '../../src/services/api'
import { renderWithAuth, VERIFIED_BUYER } from '../helpers/renderWithAuth'
import { callsTo, stubApi } from '../helpers/fakeApi'

const LISTING = {
  id: 'l-1',
  food_description: 'Vegetable lasagne',
  photo_url: null,
  pickup_address_text: 'Hoofdstraat 12, Amsterdam',
  pickup_window_start: '2026-08-04T17:00:00Z',
  pickup_window_end: '2026-08-04T19:00:00Z',
  cook_display_name: 'De Buurtkok',
}

function claim(over: Partial<ClaimItem>): ClaimItem {
  return {
    id: 'c-1',
    quantity: 2,
    amount_paid_cents: 700,
    payment_status: 'paid',
    pickup_status: 'collected',
    rating: null,
    created_at: '2026-08-04T16:00:00Z',
    listing: LISTING,
    ...over,
  }
}

function renderOrders(route = '/orders') {
  return renderWithAuth(
    <Routes>
      <Route path="/orders" element={<OrderHistoryPage />} />
      <Route path="/orders/:orderId" element={<OrderHistoryPage />} />
    </Routes>,
    { account: VERIFIED_BUYER, route },
  )
}

afterEach(() => vi.unstubAllGlobals())

describe('order history from GET /claims', () => {
  it('lists orders mapped from the claim items', async () => {
    stubApi({
      'GET /api/v1/claims': { status: 200, body: { claims: [claim({ pickup_status: 'pending' })], next_cursor: null } },
    })
    renderOrders()
    expect(await screen.findByText('Vegetable lasagne')).toBeInTheDocument()
    expect(screen.getByText(/2 portions/)).toBeInTheDocument()
    expect(screen.queryByTestId('rating-prompt')).not.toBeInTheDocument()
  })

  it('shows an empty state with no orders', async () => {
    stubApi({ 'GET /api/v1/claims': { status: 200, body: { claims: [], next_cursor: null } } })
    renderOrders()
    expect(await screen.findByText(/No orders yet/i)).toBeInTheDocument()
  })

  it('opens an order from GET /claims/{id}', async () => {
    stubApi({ 'GET /api/v1/claims/c-1': { status: 200, body: claim({ pickup_status: 'pending' }) } })
    renderOrders('/orders/c-1')
    expect(await screen.findByTestId('order-confirmation')).toBeInTheDocument()
    expect(screen.getByText('Hoofdstraat 12, Amsterdam')).toBeInTheDocument()
  })
})

describe('rating prompt', () => {
  async function openCollected(rating: ClaimItem['rating'] = null) {
    const api = stubApi({
      'GET /api/v1/claims/c-1': { status: 200, body: claim({ rating }) },
    })
    renderOrders('/orders/c-1')
    await screen.findByTestId('order-confirmation')
    return api
  }

  it('is shown for collected, unrated orders and posts the chosen stars', async () => {
    const api = await openCollected()
    api.mockClear()
    stubApi({
      'POST /api/v1/claims/c-1/rating': {
        status: 201,
        body: { claim_id: 'c-1', stars: 4, rated_at: '2026-08-04T20:00:00Z' },
      },
    })
    const user = userEvent.setup()
    const send = screen.getByRole('button', { name: 'Send rating' })
    expect(send).toBeDisabled()
    await user.click(screen.getByRole('radio', { name: '4 stars' }))
    await user.click(send)

    expect(await screen.findByTestId('rating-done')).toHaveTextContent('You rated this order 4 stars')
  })

  it('sends { stars } to the rating endpoint', async () => {
    await openCollected()
    const post = stubApi({
      'POST /api/v1/claims/c-1/rating': {
        status: 201,
        body: { claim_id: 'c-1', stars: 5, rated_at: 'x' },
      },
    })
    const user = userEvent.setup()
    await user.click(screen.getByRole('radio', { name: '5 stars' }))
    await user.click(screen.getByRole('button', { name: 'Send rating' }))
    await waitFor(() => expect(callsTo(post, 'POST', '/api/v1/claims/c-1/rating')).toHaveLength(1))
    expect(JSON.parse(String((callsTo(post, 'POST', '/api/v1/claims/c-1/rating')[0][1] as RequestInit).body))).toEqual({
      stars: 5,
    })
  })

  it('shows the existing rating instead of a prompt', async () => {
    await openCollected(3)
    expect(screen.getByTestId('rating-done')).toHaveTextContent('3 stars')
    expect(screen.queryByTestId('rating-prompt')).not.toBeInTheDocument()
  })

  it('treats 409 already_rated as done, not as an error', async () => {
    await openCollected()
    stubApi({ 'POST /api/v1/claims/c-1/rating': { status: 409, body: { error: 'already_rated' } } })
    const user = userEvent.setup()
    await user.click(screen.getByRole('radio', { name: '2 stars' }))
    await user.click(screen.getByRole('button', { name: 'Send rating' }))
    expect(await screen.findByTestId('rating-done')).toHaveTextContent(/already rated/i)
    expect(screen.queryByTestId('rating-error')).not.toBeInTheDocument()
  })

  it('explains a 403 (own listing) and keeps the prompt', async () => {
    await openCollected()
    stubApi({ 'POST /api/v1/claims/c-1/rating': { status: 403, body: { error: 'forbidden' } } })
    const user = userEvent.setup()
    await user.click(screen.getByRole('radio', { name: '1 star' }))
    await user.click(screen.getByRole('button', { name: 'Send rating' }))
    expect(await screen.findByTestId('rating-error')).toHaveTextContent(/can't rate this order/i)
    expect(screen.getByTestId('rating-prompt')).toBeInTheDocument()
  })

  it('is not offered before pickup', async () => {
    stubApi({ 'GET /api/v1/claims/c-1': { status: 200, body: claim({ pickup_status: 'pending' }) } })
    renderOrders('/orders/c-1')
    await screen.findByTestId('order-confirmation')
    expect(screen.queryByTestId('rating-prompt')).not.toBeInTheDocument()
  })
})

describe('mark as collected', () => {
  const open = () => ({
    pickup_window_start: new Date(Date.now() - 3_600_000).toISOString(),
    pickup_window_end: new Date(Date.now() + 3_600_000).toISOString(),
  })
  const paidPending = (over: Partial<ClaimItem['listing']> = {}, c: Partial<ClaimItem> = {}) =>
    claim({ pickup_status: 'pending', listing: { ...LISTING, ...open(), ...over }, ...c })

  it('confirms pickup, then the rating prompt appears', async () => {
    let collected = false
    const api = stubApi({
      'GET /api/v1/claims/c-1': () => ({
        status: 200,
        body: paidPending({}, { pickup_status: collected ? 'collected' : 'pending' }),
      }),
      'POST /api/v1/claims/c-1/confirm-pickup': () => {
        collected = true
        return { status: 200, body: { pickup_status: 'collected' } }
      },
    })
    renderOrders('/orders/c-1')
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Mark as collected' }))
    expect(await screen.findByTestId('rating-prompt')).toBeInTheDocument()
    expect(callsTo(api, 'POST', '/api/v1/claims/c-1/confirm-pickup')).toHaveLength(1)
    expect(screen.queryByRole('button', { name: 'Mark as collected' })).not.toBeInTheDocument()
  })

  it('is also offered in the order list', async () => {
    stubApi({ 'GET /api/v1/claims': { status: 200, body: { claims: [paidPending()], next_cursor: null } } })
    renderOrders()
    expect(await screen.findByRole('button', { name: 'Mark as collected' })).toBeInTheDocument()
  })

  it('409 claim_conflict explains the listing was cancelled', async () => {
    stubApi({
      'GET /api/v1/claims/c-1': { status: 200, body: paidPending() },
      'POST /api/v1/claims/c-1/confirm-pickup': { status: 409, body: { error: 'claim_conflict' } },
    })
    renderOrders('/orders/c-1')
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Mark as collected' }))
    expect(await screen.findByTestId('collect-error')).toHaveTextContent(/cook cancelled/i)
  })

  it('is hidden outside the pickup window, for unpaid orders and once collected', async () => {
    const closed = paidPending({
      pickup_window_start: new Date(Date.now() - 7_200_000).toISOString(),
      pickup_window_end: new Date(Date.now() - 3_600_000).toISOString(),
    })
    stubApi({
      'GET /api/v1/claims': {
        status: 200,
        body: {
          claims: [
            closed,
            paidPending({}, { id: 'c-2', payment_status: 'pending_payment' }),
            claim({ id: 'c-3', pickup_status: 'collected', rating: 4 }),
          ],
          next_cursor: null,
        },
      },
    })
    renderOrders()
    await screen.findAllByRole('listitem')
    expect(screen.queryByRole('button', { name: 'Mark as collected' })).not.toBeInTheDocument()
  })
})

describe('order history pagination', () => {
  it('loads the next page with the cursor', async () => {
    const api = stubApi({
      'GET /api/v1/claims': ({ url }) =>
        url.includes('cursor=p2')
          ? { status: 200, body: { claims: [claim({ id: 'c-2', listing: { ...LISTING, food_description: 'Second dish' } })], next_cursor: null } }
          : { status: 200, body: { claims: [claim({ id: 'c-1' })], next_cursor: 'p2' } },
    })
    renderOrders()
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Load more' }))
    expect(await screen.findByText('Second dish')).toBeInTheDocument()
    expect(screen.getByText('Vegetable lasagne')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Load more' })).not.toBeInTheDocument()
    expect(callsTo(api, 'GET', '/api/v1/claims')).toHaveLength(2)
  })
})
