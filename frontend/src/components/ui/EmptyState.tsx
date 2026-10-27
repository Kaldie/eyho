import type { ReactNode } from 'react'

export interface EmptyStateProps {
  /** Icon/illustration slot, e.g. an inline SVG or emoji. */
  icon?: ReactNode
  title: string
  description?: string
  /** CTA slot, typically a <Button> or <Link>. */
  action?: ReactNode
  className?: string
}

/**
 * Inviting empty state (FR-010) — used when a discovery query returns zero listings, or
 * a list view (order history, cook dashboard) has nothing to show yet. Never a bare
 * "No results" message; always paired with a next-step CTA when one is available.
 */
export function EmptyState({ icon, title, description, action, className = '' }: EmptyStateProps) {
  return (
    <div
      className={[
        'flex flex-col items-center justify-center gap-sm rounded-card border border-dashed',
        'border-neutral-300 bg-neutral-50 px-lg py-2xl text-center',
        className,
      ]
        .filter(Boolean)
        .join(' ')}
    >
      {icon && (
        <div className="text-4xl" aria-hidden="true">
          {icon}
        </div>
      )}
      <p className="text-lg font-semibold text-neutral-800">{title}</p>
      {description && <p className="max-w-[24rem] text-sm text-neutral-500">{description}</p>}
      {action && <div className="mt-sm">{action}</div>}
    </div>
  )
}
