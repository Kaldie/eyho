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
  const fmt = (iso: string) =>
    new Date(iso).toLocaleString('nl-NL', {
      dateStyle: 'medium',
      timeStyle: 'short',
    })
  return `${fmt(start)} – ${fmt(end)}`
}

/**
 * Reusable order/pickup confirmation display (FR-005). Shown at the end of checkout
 * (ClaimFlow, T030) and re-rendered later from Order History (T031) so a buyer can
 * retrieve the same confirmation again.
 */
export function OrderConfirmation({ order, action }: OrderConfirmationProps) {
  return (
    <div data-testid="order-confirmation" className="flex flex-col gap-md">
      <div className="flex items-center gap-sm text-primary-700">
        <span aria-hidden="true" className="text-2xl">✅</span>
        <h3 className="text-lg font-bold">Pickup confirmed</h3>
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
          <dd>{formatWindow(order.pickupWindow.start, order.pickupWindow.end)}</dd>
        </div>
      </dl>

      <p className="text-xs text-neutral-400">
        Order ID: <code>{order.orderId}</code>
      </p>

      {action}
    </div>
  )
}
