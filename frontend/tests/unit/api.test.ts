import { describe, it, expect, afterEach, vi } from 'vitest'
import {
  ApiError,
  createClaim,
  getCookListings,
  getListing,
  getListings,
  getOrdersPage,
  toOrderSummary,
} from '../../src/services/api'
import { callsTo, stubApi } from '../helpers/fakeApi'

afterEach(() => vi.unstubAllGlobals())

describe('services/api', () => {
  it('getListings sends lat, lng and radius_km and unwraps { listings }', async () => {
    const api = stubApi({ 'GET /api/v1/listings': { status: 200, body: { listings: [{ id: 'a' }] } } })
    const result = await getListings(52.37, 4.9, 5)
    expect(result).toEqual([{ id: 'a' }])
    const url = new URL(String(api.mock.calls[0][0]), 'http://x')
    expect(Object.fromEntries(url.searchParams)).toEqual({ lat: '52.37', lng: '4.9', radius_km: '5' })
  })

  it('getListing sends lat and lng together only when given', async () => {
    const api = stubApi({ 'GET /api/v1/listings/l-1': { status: 200, body: { id: 'l-1' } } })
    await getListing('l-1')
    await getListing('l-1', { lat: 52.37, lng: 4.9 })
    expect(String(api.mock.calls[0][0])).toBe('/api/v1/listings/l-1')
    expect(String(api.mock.calls[1][0])).toBe('/api/v1/listings/l-1?lat=52.37&lng=4.9')
  })

  it('getCookListings reads GET /me/listings', async () => {
    stubApi({ 'GET /api/v1/me/listings': { status: 200, body: { listings: [{ id: 'a', portions_claimed: 1 }] } } })
    expect(await getCookListings()).toEqual([{ id: 'a', portions_claimed: 1 }])
  })

  it('getOrdersPage passes the cursor and returns next_cursor', async () => {
    const api = stubApi({ 'GET /api/v1/claims': { status: 200, body: { claims: [], next_cursor: 'abc' } } })
    const page = await getOrdersPage('xyz')
    expect(String(api.mock.calls[0][0])).toBe('/api/v1/claims?cursor=xyz')
    expect(page.nextCursor).toBe('abc')
  })

  it('createClaim posts listing_id and quantity and throws ApiError with the contract body on 409', async () => {
    const api = stubApi({
      'POST /api/v1/claims': { status: 409, body: { error: 'insufficient_portions', portions_remaining: 1 } },
    })
    const err = await createClaim('l-1', 3).catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 409, code: 'insufficient_portions', body: { portions_remaining: 1 } })
    expect(JSON.parse(String((callsTo(api, 'POST', '/api/v1/claims')[0][1] as RequestInit).body))).toEqual({
      listing_id: 'l-1',
      quantity: 3,
    })
  })

  it('getOrdersPage maps claim items through toOrderSummary', async () => {
    const claim = {
      id: 'c-1',
      quantity: 1,
      amount_paid_cents: 350,
      payment_status: 'paid',
      pickup_status: 'collected',
      rating: 5,
      created_at: '2026-08-04T17:05:00Z',
      listing: {
        id: 'l-1',
        food_description: 'Soup',
        photo_url: null,
        pickup_address_text: 'Street 1',
        pickup_window_start: 's',
        pickup_window_end: 'e',
        cook_display_name: 'Cook',
      },
    }
    stubApi({ 'GET /api/v1/claims': { status: 200, body: { claims: [claim], next_cursor: null } } })
    const page = await getOrdersPage()
    const [order] = page.orders
    expect(page.nextCursor).toBeNull()
    expect(order).toEqual(toOrderSummary(claim as Parameters<typeof toOrderSummary>[0]))
    expect(order).toMatchObject({ orderId: 'c-1', totalPrice: 350, pickupLocation: { label: 'Cook', address: 'Street 1' }, rating: 5 })
  })
})
