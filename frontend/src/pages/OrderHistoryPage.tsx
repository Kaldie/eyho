import { Link, useParams } from 'react-router-dom'
import { useOrder, useOrders } from '@/services/api'
import { OrderConfirmation } from '@/components/OrderConfirmation'
import { EmptyState } from '@/components/ui/EmptyState'
import { Skeleton } from '@/components/ui/Skeleton'
import { Button } from '@/components/ui/Button'
import { ImageWithFallback } from '@/components/ui/ImageWithFallback'

function formatPrice(cents: number): string {
  return new Intl.NumberFormat('nl-NL', { style: 'currency', currency: 'EUR' }).format(
    cents / 100,
  )
}

/**
 * OrderHistoryPage — lists the buyer's past claims/orders (FR-005), each linking to a
 * confirmation view reusing the same `OrderConfirmation` shown at the end of checkout
 * so a buyer can retrieve their pickup details again later.
 */
export function OrderHistoryPage() {
  const { orderId } = useParams<{ orderId: string }>()
  return orderId ? <OrderDetail orderId={orderId} /> : <OrderList />
}

function OrderList() {
  const { data: orders = [], isLoading, isError } = useOrders()

  return (
    <main className="mx-auto max-w-[42rem] px-md py-lg sm:px-lg">
      <h1 className="mb-lg text-2xl font-bold text-neutral-900">Order history</h1>

      {isLoading && (
        <div className="flex flex-col gap-md">
          <Skeleton className="h-24 w-full" label="Loading orders" />
          <Skeleton className="h-24 w-full" label="Loading orders" />
        </div>
      )}

      {isError && <p className="text-neutral-700">Failed to load your orders.</p>}

      {!isLoading && !isError && orders.length === 0 && (
        <EmptyState
          icon="🧾"
          title="No orders yet"
          description="Claimed listings and their pickup details will show up here."
          action={
            <Link to="/">
              <Button type="button">Browse nearby food</Button>
            </Link>
          }
        />
      )}

      {!isLoading && !isError && orders.length > 0 && (
        <ul className="flex flex-col gap-md">
          {orders.map((order) => (
            <li key={order.orderId}>
              <Link
                to={`/orders/${order.orderId}`}
                className="flex items-center gap-md rounded-card border border-neutral-200 p-md hover:bg-neutral-50"
              >
                <ImageWithFallback
                  src={order.listingSummary.photoUrl}
                  alt={order.listingSummary.foodName}
                  className="h-16 w-16 shrink-0"
                  aspectRatio="1 / 1"
                />
                <div className="min-w-0">
                  <p className="truncate font-semibold text-neutral-900">
                    {order.listingSummary.foodName}
                  </p>
                  <p className="text-sm text-neutral-600">
                    {order.quantity} portion{order.quantity === 1 ? '' : 's'} ·{' '}
                    {formatPrice(order.totalPrice)}
                  </p>
                </div>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </main>
  )
}

function OrderDetail({ orderId }: { orderId: string }) {
  const { data: order, isLoading, isError } = useOrder(orderId)

  return (
    <main className="mx-auto max-w-[42rem] px-md py-lg sm:px-lg">
      <Link to="/orders" className="mb-md inline-block text-sm font-semibold text-primary-700">
        &larr; Back to order history
      </Link>

      {isLoading && <Skeleton className="h-64 w-full" label="Loading order" />}

      {isError || (!isLoading && !order) ? (
        <p className="text-neutral-700">Order not found.</p>
      ) : null}

      {order && <OrderConfirmation order={order} />}
    </main>
  )
}
