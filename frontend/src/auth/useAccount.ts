import { useQuery } from '@tanstack/react-query'
import { ME_QUERY_KEY, getMe, type Account } from '@/services/accounts'
import { useSession, type SessionStatus } from '@/auth/session'

export interface UseAccountResult {
  status: SessionStatus
  /** The signed-in account from GET /me; undefined while signed out or loading. */
  account: Account | undefined
  /** True while the session or the account is still being resolved. */
  isLoading: boolean
  isError: boolean
  refetch: () => Promise<unknown>
}

/** The current account (GET /me), fetched only while signed in. */
export function useAccount(): UseAccountResult {
  const { status } = useSession()
  const query = useQuery({
    queryKey: ME_QUERY_KEY,
    queryFn: getMe,
    enabled: status === 'signedIn',
    staleTime: 60_000,
    retry: false,
  })
  return {
    status,
    account: status === 'signedIn' ? query.data : undefined,
    isLoading: status === 'loading' || (status === 'signedIn' && query.isLoading),
    isError: status === 'signedIn' && query.isError,
    refetch: () => query.refetch(),
  }
}
