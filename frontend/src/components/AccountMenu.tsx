import { useEffect, useRef, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useAccount } from '@/auth/useAccount'
import { useSession } from '@/auth/session'

/**
 * Header account control (FR-014). Signed out: a "Sign in" link. Signed in: the person's
 * name opening a menu with Account and Sign out.
 */
export function AccountMenu() {
  const session = useSession()
  const { account } = useAccount()
  const location = useLocation()
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)

  // Close on route change, outside click and Escape.
  useEffect(() => setOpen(false), [location.pathname])
  useEffect(() => {
    if (!open) return
    function onPointer(e: MouseEvent) {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false)
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onPointer)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onPointer)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  if (session.status === 'loading') {
    return <span className="px-sm text-sm text-neutral-500" aria-hidden="true" />
  }

  if (session.status === 'signedOut') {
    return (
      <Link
        to="/sign-in"
        state={{ returnTo: `${location.pathname}${location.search}` }}
        className="rounded-control bg-primary-600 px-md py-xs text-sm font-semibold text-white no-underline transition-colors hover:bg-primary-700 motion-reduce:transition-none"
      >
        Sign in
      </Link>
    )
  }

  const name = account?.display_name || account?.email || 'Account'

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        className="max-w-[10rem] truncate rounded-control border border-neutral-300 bg-white px-md py-xs text-sm font-semibold text-neutral-900 hover:border-neutral-400"
      >
        {name}
      </button>
      {open && (
        <div
          role="menu"
          aria-label="Account"
          className="absolute right-0 mt-xs flex min-w-[10rem] flex-col rounded-card border border-neutral-200 bg-white p-xs shadow-lg"
        >
          <Link
            role="menuitem"
            to="/account"
            className="rounded-control px-md py-sm text-sm font-semibold text-neutral-800 no-underline hover:bg-neutral-100"
          >
            Account
          </Link>
          <button
            type="button"
            role="menuitem"
            onClick={() => void session.signOut()}
            className="rounded-control px-md py-sm text-left text-sm font-semibold text-neutral-800 hover:bg-neutral-100"
          >
            Sign out
          </button>
        </div>
      )}
    </div>
  )
}
