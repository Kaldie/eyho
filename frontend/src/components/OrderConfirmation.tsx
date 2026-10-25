import type { OrderConfirmationView } from '@/types/views'
import { ImageWithFallback } from '@/components/ui/ImageWithFallback'

export interface OrderConfirmationProps {
  order: OrderConfirmationView
  /** Optional trailing action slot, e.g. "Done" (ClaimFlow) or "Back to orders" (history). */
  action?: React.ReactNode
}

function formatPrice(cents: number): string {
  return new Intl.NumberFormat('nl-NL', { style: 'currency', currency: 'EUR' }).format(
    cents / 100,
  )
}

function formatWindow(start: string, end: string): string {
  const day = new Date(start).toLocaleDateString('en-GB', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
  })
  const time = (iso: string) =>
    new Date(iso).toLocaleTimeString('nl-NL', { hour: '2-digit', minute: '2-digit' })
  return `${day}, ${time(start)}–${time(end)}`
}

/**
 * Reusable order/pickup confirmation display (FR-005). Shown at the end of checkout
 * (ClaimFlow, T030) and re-rendered later from Order History (T031) so a buyer can
 * retrieve the same confirmation again.
 */
export function OrderConfirmation({ order, action }: OrderConfirmationProps) {
  return (
    <div data-testid="order-confirmation" className="flex flex-col gap-md">
      <div>
        <h3 className="text-xl font-extrabold text-primary-700">It&rsquo;s yours</h3>
        <p className="text-sm text-neutral-600">
          Pick it up during the window below. The cook is expecting you.
        </p>
      </div>

      <div className="flex gap-md">
        <ImageWithFallback
          src={order.listingSummary.photoUrl}
          alt={order.listingSummary.foodName}
          className="h-20 w-20 shrink-0"
          aspectRatio="1 / 1"
        />
        <div>
          <p className="font-semibold text-neutral-900">{order.listingSummary.foodName}</p>
          <p className="text-sm text-neutral-600">
            {order.quantity} portion{order.quantity === 1 ? '' : 's'} · {formatPrice(order.totalPrice)}
          </p>
        </div>
      </div>

      <dl className="grid grid-cols-1 gap-xs text-sm text-neutral-700 sm:grid-cols-2">
        <div>
          <dt className="font-semibold text-neutral-900">Pickup location</dt>
          <dd>{order.pickupLocation.label}</dd>
          <dd className="text-neutral-500">{order.pickupLocation.address}</dd>
        </div>
        <div>
          <dt className="font-semibold text-neutral-900">Pickup window</dt>
          <dd className="tabular-nums">{formatWindow(order.pickupWindow.start, order.pickupWindow.end)}</dd>
        </div>
      </dl>

      <p className="text-xs text-neutral-500">
        Order ID: <code>{order.orderId}</code>
      </p>

      {action}
    </div>
  )
}
