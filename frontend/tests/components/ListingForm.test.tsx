/**
 * Component tests for ListingForm — verifies the live buyer-facing preview
 * (rendered via ListingCard) updates as form fields change (FR-007).
 */
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ListingForm } from '../../src/components/ListingForm'
import type { CookSummary } from '../../src/services/api'

const COOK: CookSummary = {
  display_name: 'Test Cook',
  org_type: 'nonprofit',
  verification_status: 'unverified',
  rating_avg: 4.2,
}

describe('ListingForm — live preview', () => {
  it('updates the preview food description as the user types', async () => {
    const user = userEvent.setup()
    render(<ListingForm cook={COOK} />)

    const input = screen.getByRole('textbox', { name: /food description/i })
    await user.type(input, 'Vegetable lasagne')

    expect(screen.getByRole('heading', { name: 'Vegetable lasagne' })).toBeTruthy()
  })

  it('updates the preview price as the user types', async () => {
    const user = userEvent.setup()
    render(<ListingForm cook={COOK} />)

    const priceInput = screen.getByLabelText(/reduced price/i)
    await user.clear(priceInput)
    await user.type(priceInput, '450')

    expect(screen.getByText(/€\s*4,50/)).toBeTruthy()
  })

  it('updates the preview portions remaining as the user types', async () => {
    const user = userEvent.setup()
    render(<ListingForm cook={COOK} />)

    const portionsInput = screen.getByLabelText(/portions available/i)
    await user.clear(portionsInput)
    await user.type(portionsInput, '7')

    expect(screen.getByText(/7 portions left/)).toBeTruthy()
  })

  it('always shows the verification badge in the preview, regardless of form state', () => {
    render(<ListingForm cook={COOK} />)
    expect(screen.getByTestId('verification-badge').textContent).toBe('unverified')
  })
})
