export interface SkeletonProps {
  className?: string
  /** Accessible label announced while content is loading (FR-009). */
  label?: string
}

/**
 * Animated placeholder shown while content (images, cards, lists) is loading (FR-009).
 * Respects `prefers-reduced-motion` via the `motion-reduce:animate-none` utility.
 */
export function Skeleton({ className = '', label = 'Loading' }: SkeletonProps) {
  return (
    <div
      role="status"
      aria-label={label}
      className={[
        'animate-pulse motion-reduce:animate-none rounded-card bg-neutral-200',
        className,
      ]
        .filter(Boolean)
        .join(' ')}
    />
  )
}
