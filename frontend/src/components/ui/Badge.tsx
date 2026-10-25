import type { HTMLAttributes, ReactNode } from 'react'

export type BadgeTone = 'neutral' | 'success' | 'warning'

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  children: ReactNode
  tone?: BadgeTone
  className?: string
}

// Verification-status and listing-status badges use neutral (non-alarming) styling by
// default per Constitution Principle I and the spec's Edge Cases — an "unverified" cook
// or a "sold out" listing is information, not a warning, so it must never look alarming.
const TONE_CLASSES: Record<BadgeTone, string> = {
  neutral: 'bg-neutral-100 text-neutral-700 border border-neutral-200',
  success: 'bg-primary-50 text-primary-700 border border-primary-200',
  warning: 'bg-neutral-100 text-neutral-600 border border-neutral-300',
}

/**
 * Badge primitive for verification-status and listing-status labels (FR-004, FR-006).
 * Deliberately avoids red/error styling — status is always shown, never hidden, and never
 * dramatized (Principle I: crowd-sourced verification labels content, it doesn't gate it).
 */
export function Badge({ children, tone = 'neutral', className = '', ...rest }: BadgeProps) {
  const classes = [
    'inline-flex items-center rounded-full px-sm py-xs text-xs font-medium leading-none',
    TONE_CLASSES[tone],
    className,
  ]
    .filter(Boolean)
    .join(' ')
  return (
    <span className={classes} role="status" {...rest}>
      {children}
    </span>
  )
}
