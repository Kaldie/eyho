import { Routes, Route, Link } from 'react-router-dom'
import { DiscoveryPage } from '@/pages/DiscoveryPage'
import { ListingDetailPage } from '@/pages/ListingDetailPage'
import { CookDashboardPage } from '@/pages/CookDashboardPage'
import { OrderHistoryPage } from '@/pages/OrderHistoryPage'

export function App() {
  return (
    <>
      <nav className="flex items-center gap-lg border-b border-neutral-200 bg-white px-md py-sm">
        <Link to="/" className="text-lg font-bold text-primary-700 no-underline">
          Eyho
        </Link>
        <Link to="/" className="text-sm text-neutral-600 no-underline hover:text-neutral-900">
          Discover
        </Link>
        <Link to="/dashboard" className="text-sm text-neutral-600 no-underline hover:text-neutral-900">
          Cook Dashboard
        </Link>
        <Link to="/orders" className="text-sm text-neutral-600 no-underline hover:text-neutral-900">
          Order History
        </Link>
      </nav>

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
