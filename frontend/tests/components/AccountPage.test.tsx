import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AccountPage, deletionBlockedMessage } from '../../src/pages/AccountPage'
import { ApiError } from '../../src/services/api'
import * as accounts from '../../src/services/accounts'
import { renderWithAuth, VERIFIED_BUYER } from '../helpers/renderWithAuth'

beforeEach(() => vi.restoreAllMocks())

async function startDeletion() {
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: 'Delete my account' }))
  await user.click(screen.getByRole('button', { name: /Yes, delete my account/i }))
}

describe('deletionBlockedMessage', () => {
  it('explains active_claims and active_listings with counts', () => {
    expect(deletionBlockedMessage(new ApiError(409, { error: 'active_claims', count: 2 }))).toMatch(
      /2 claims waiting for pickup/,
    )
    expect(deletionBlockedMessage(new ApiError(409, { error: 'active_listings', count: 1 }))).toMatch(
      /One of your listings/,
    )
  })

  it('falls back to a generic message', () => {
    expect(deletionBlockedMessage(new Error('network'))).toMatch(/try again/i)
  })
})

describe('AccountPage deletion', () => {
  it('asks for confirmation before deleting', async () => {
    const del = vi.spyOn(accounts, 'deleteMe')
    const user = userEvent.setup()
    renderWithAuth(<AccountPage />, { account: VERIFIED_BUYER })

    await user.click(screen.getByRole('button', { name: 'Delete my account' }))
    expect(screen.getByRole('alertdialog')).toBeInTheDocument()
    expect(del).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: 'Keep my account' }))
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('shows the active_claims explanation on 409', async () => {
    vi.spyOn(accounts, 'deleteMe').mockRejectedValue(
      new ApiError(409, { error: 'active_claims', count: 2 }),
    )
    renderWithAuth(<AccountPage />, { account: VERIFIED_BUYER })
    await startDeletion()
    expect(await screen.findByTestId('delete-blocked')).toHaveTextContent(
      /2 claims waiting for pickup/,
    )
  })

  it('shows the active_listings explanation on 409', async () => {
    vi.spyOn(accounts, 'deleteMe').mockRejectedValue(
      new ApiError(409, { error: 'active_listings', count: 1 }),
    )
    renderWithAuth(<AccountPage />, { account: VERIFIED_BUYER })
    await startDeletion()
    expect(await screen.findByTestId('delete-blocked')).toHaveTextContent(/One of your listings/)
  })

  it('signs out after a successful deletion', async () => {
    vi.spyOn(accounts, 'deleteMe').mockResolvedValue(undefined)
    const signOut = vi.fn().mockResolvedValue(undefined)
    renderWithAuth(<AccountPage />, { account: VERIFIED_BUYER, session: { signOut } })
    await startDeletion()
    await vi.waitFor(() => expect(signOut).toHaveBeenCalled())
  })
})
