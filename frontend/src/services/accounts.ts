// Typed client for contracts/accounts-api.md (feature 001). Shapes follow the contract
// exactly; do not add fields here that the backend does not serve.

import { useQuery } from '@tanstack/react-query'
import { ApiError, apiFetch, readJson } from '@/services/api'
import type { OrgType, VerificationStatus } from '@/services/api'

export type RegistrationStatus = 'pending' | 'active' | 'inactive'

export interface CookProfile {
  id: string
  display_name: string
  org_type: OrgType
  phone: string
  default_pickup_address_text: string
  default_pickup_location: { lat: number; lng: number }
  verification_status: VerificationStatus
  kvk_number: string
  kvk_trade_name: string | null
  registration_status: RegistrationStatus
  registration_checked_at: string | null
  nvwa_declared_at: string | null
  can_publish: boolean
}

export interface Account {
  id: string
  email: string | null
  email_verified: boolean
  display_name: string | null
  is_demo: boolean
  cook_profile: CookProfile | null
}

export interface CookProfileInput {
  display_name: string
  org_type: OrgType
  phone: string
  default_pickup_address_text: string
  default_pickup_location: { lat: number; lng: number }
  kvk_number: string
  nvwa_declaration: { accepted: true; version: string }
}

export interface DemoAccount {
  key: string
  label: string
}

export interface DemoToken {
  access_token: string
  expires_at: string
}

async function expectOk(res: Response): Promise<Response> {
  if (!res.ok) throw new ApiError(res.status, await readJson(res))
  return res
}

/** GET /me */
export async function getMe(): Promise<Account> {
  const res = await expectOk(await apiFetch('/api/v1/me'))
  return (await res.json()) as Account
}

/** PATCH /me — updates display_name. */
export async function updateMe(input: { display_name: string }): Promise<Account> {
  const res = await expectOk(
    await apiFetch('/api/v1/me', { method: 'PATCH', body: JSON.stringify(input) }),
  )
  return (await res.json()) as Account
}

/** DELETE /me — 204, or 409 `active_claims` / `active_listings` with a `count`. */
export async function deleteMe(): Promise<void> {
  await expectOk(await apiFetch('/api/v1/me', { method: 'DELETE' }))
}

/** POST /me/verification-email — 202; 409 `already_verified`; 429 when called twice a minute. */
export async function resendVerificationEmail(): Promise<void> {
  await expectOk(await apiFetch('/api/v1/me/verification-email', { method: 'POST' }))
}

export interface CookProfileResult {
  profile: CookProfile
  /** 201 = registration confirmed; 202 = register unreachable, check retried in background. */
  status: 201 | 202 | 200
}

/** POST /me/cook-profile */
export async function createCookProfile(input: CookProfileInput): Promise<CookProfileResult> {
  const res = await expectOk(
    await apiFetch('/api/v1/me/cook-profile', { method: 'POST', body: JSON.stringify(input) }),
  )
  return { profile: (await res.json()) as CookProfile, status: res.status as 201 | 202 }
}

/** PATCH /me/cook-profile */
export async function updateCookProfile(
  input: Partial<Omit<CookProfileInput, 'nvwa_declaration'>>,
): Promise<CookProfileResult> {
  const res = await expectOk(
    await apiFetch('/api/v1/me/cook-profile', { method: 'PATCH', body: JSON.stringify(input) }),
  )
  return { profile: (await res.json()) as CookProfile, status: res.status as 200 | 202 }
}

/**
 * GET /demo/accounts — only routed in demo/local. Resolves to `[]` when the endpoint is
 * absent or unreachable so the UI simply hides the demo buttons.
 */
export async function getDemoAccounts(): Promise<DemoAccount[]> {
  try {
    const res = await apiFetch('/api/v1/demo/accounts')
    if (!res.ok) return []
    const body = (await res.json()) as { accounts?: DemoAccount[] }
    return Array.isArray(body.accounts) ? body.accounts : []
  } catch {
    return []
  }
}

/** POST /demo/sign-in */
export async function demoSignIn(key: string): Promise<DemoToken> {
  const res = await expectOk(
    await apiFetch('/api/v1/demo/sign-in', { method: 'POST', body: JSON.stringify({ key }) }),
  )
  return (await res.json()) as DemoToken
}

export const ME_QUERY_KEY = ['me'] as const

export function useDemoAccounts() {
  return useQuery({
    queryKey: ['demo-accounts'],
    queryFn: getDemoAccounts,
    staleTime: 5 * 60_000,
    retry: false,
  })
}
