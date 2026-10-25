import { Routes, Route, NavLink, Link } from 'react-router-dom'
import { DiscoveryPage } from '@/pages/DiscoveryPage'
import { ListingDetailPage } from '@/pages/ListingDetailPage'
import { CookDashboardPage } from '@/pages/CookDashboardPage'
import { OrderHistoryPage } from '@/pages/OrderHistoryPage'

const NAV_ITEMS = [
  { to: '/', label: 'Food', end: true },
  { to: '/orders', label: 'Orders', end: false },
  { to: '/dashboard', label: 'For cooks', end: false },
]

export function App() {
  return (
    <>
      <header className="sticky top-0 z-10 border-b border-neutral-200 bg-[rgb(247_248_246/0.92)] backdrop-blur">
        <nav
          aria-label="Main"
          className="mx-auto flex max-w-6xl items-center gap-md px-md py-sm sm:px-lg"
        >
          <Link
            to="/"
            className="mr-auto text-xl font-extrabold tracking-tight text-primary-700 no-underline"
          >
            eyho
          </Link>
          {NAV_ITEMS.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) =>
                [
                  'rounded-control px-sm py-xs text-sm font-semibold no-underline transition-colors motion-reduce:transition-none',
                  isActive
                    ? 'bg-primary-100 text-primary-800'
                    : 'text-neutral-600 hover:text-neutral-900',
                ].join(' ')
              }
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
      </header>

      <Routes>
        <Route path="/" element={<DiscoveryPage />} />
        <Route path="/listings/:id" element={<ListingDetailPage />} />
        <Route path="/dashboard" element={<CookDashboardPage />} />
        <Route path="/orders" element={<OrderHistoryPage />} />
        <Route path="/orders/:orderId" element={<OrderHistoryPage />} />
      </Routes>
    </>
  )
}
