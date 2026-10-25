import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  CookOnboardingPage,
  describeOnboardingError,
  validateOnboarding,
  type OnboardingValues,
} from '../../src/pages/CookOnboardingPage'
import { RegistrationStatusCard } from '../../src/pages/CookDashboardPage'
import { ApiError } from '../../src/services/api'
import * as accounts from '../../src/services/accounts'
import { parsePoint } from '../../src/components/AddressSearch'
import { renderWithAuth, VERIFIED_BUYER } from '../helpers/renderWithAuth'

const VALID: OnboardingValues = {
  displayName: "Sanne's Kitchen",
  orgType: 'professional',
  phone: '+31612345678',
  address: { text: 'Keizersgracht 88, Amsterdam', lat: 52.37, lng: 4.89 },
  kvkNumber: '68750110',
  declared: true,
}

describe('validateOnboarding', () => {
  it('accepts a complete form', () => {
    expect(validateOnboarding(VALID)).toEqual({})
  })

  it('reports each missing field', () => {
    const errors = validateOnboarding({
      displayName: '',
      orgType: '',
      phone: '',
      address: null,
      kvkNumber: '',
      declared: false,
    })
    expect(Object.keys(errors).sort()).toEqual(
      ['address', 'declaration', 'display_name', 'kvk_number', 'org_type', 'phone'].sort(),
    )
  })

  it('requires an 8-digit KvK number', () => {
    expect(validateOnboarding({ ...VALID, kvkNumber: '1234' }).kvk_number).toMatch(/8 digits/)
  })
})

describe('describeOnboardingError', () => {
  it('explains kvk_not_found and kvk_inactive on the KvK field', () => {
    expect(describeOnboardingError(new ApiError(422, { error: 'kvk_not_found' }))).toMatchObject({
      field: 'kvk_number',
      message: expect.stringMatching(/couldn.t find this KvK number/i),
    })
    expect(describeOnboardingError(new ApiError(422, { error: 'kvk_inactive' }))).toMatchObject({
      field: 'kvk_number',
      message: expect.stringMatching(/no longer active/i),
    })
  })

  it('maps missing_field to the right input', () => {
    expect(
      describeOnboardingError(new ApiError(422, { error: 'missing_field', field: 'phone' })),
    ).toMatchObject({ field: 'phone', message: expect.stringMatching(/phone number/i) })
  })
})

describe('CookOnboardingPage', () => {
  it('shows validation messages and does not call the API for an empty form', async () => {
    const create = vi.spyOn(accounts, 'createCookProfile')
    const user = userEvent.setup()
    renderWithAuth(<CookOnboardingPage />, { account: VERIFIED_BUYER })

    await user.click(screen.getByRole('button', { name: 'Become a cook' }))

    expect(screen.getByText(/Enter the name buyers will see/i)).toBeInTheDocument()
    expect(screen.getByText(/Choose non-profit or professional/i)).toBeInTheDocument()
    expect(screen.getByText(/Enter a phone number/i)).toBeInTheDocument()
    expect(screen.getByText(/Search for your address/i)).toBeInTheDocument()
    expect(screen.getByText(/Enter your 8-digit KvK number/i)).toBeInTheDocument()
    expect(screen.getByText(/Confirm the declaration/i)).toBeInTheDocument()
    expect(create).not.toHaveBeenCalled()
  })

  it('tells the cook up front they start unverified', () => {
    renderWithAuth(<CookOnboardingPage />, { account: VERIFIED_BUYER })
    expect(screen.getByText('unverified')).toBeInTheDocument()
  })
})

describe('RegistrationStatusCard', () => {
  const cook = {
    id: 'c1',
    display_name: 'K',
    org_type: 'professional' as const,
    phone: '',
    default_pickup_address_text: '',
    default_pickup_location: { lat: 0, lng: 0 },
    verification_status: 'unverified' as const,
    kvk_number: '68750110',
    kvk_trade_name: "Sanne's Kitchen B.V.",
    registration_status: 'active' as const,
    registration_checked_at: null,
    nvwa_declared_at: null,
    can_publish: true,
  }

  it('shows Registered with the trade name', () => {
    renderWithAuth(<RegistrationStatusCard cook={cook} />)
    expect(screen.getByText('Registered')).toBeInTheDocument()
    expect(screen.getByText(/Sanne's Kitchen B\.V\./)).toBeInTheDocument()
  })

  it('shows the pending and inactive states without using the word verified', () => {
    const { unmount } = renderWithAuth(
      <RegistrationStatusCard cook={{ ...cook, registration_status: 'pending', can_publish: false }} />,
    )
    expect(screen.getByText('Checking your registration')).toBeInTheDocument()
    unmount()
    renderWithAuth(
      <RegistrationStatusCard cook={{ ...cook, registration_status: 'inactive', can_publish: false }} />,
    )
    expect(screen.getByText('Registration inactive')).toBeInTheDocument()
    expect(screen.getByTestId('registration-card').textContent).not.toMatch(/verified/i)
  })
})

describe('parsePoint', () => {
  it('reads PDOK WKT as lat/lng', () => {
    expect(parsePoint('POINT(4.89 52.37)')).toEqual({ lat: 52.37, lng: 4.89 })
    expect(parsePoint('nonsense')).toBeNull()
  })
})
