import type { ButtonHTMLAttributes, ReactNode } from 'react'

export type ButtonVariant = 'primary' | 'secondary'

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  children: ReactNode
}

const BASE_CLASSES =
  'inline-flex min-h-[2.75rem] items-center justify-center gap-sm rounded-control px-lg py-sm font-semibold ' +
  'text-base transition-colors motion-reduce:transition-none focus-visible:outline focus-visible:outline-2 ' +
  'focus-visible:outline-offset-2 focus-visible:outline-primary-700 ' +
  'disabled:cursor-not-allowed disabled:opacity-50'

const VARIANT_CLASSES: Record<ButtonVariant, string> = {
  // Meets WCAG AA contrast: white text on primary-600 (~4.7:1) per FR-012.
  primary: 'bg-primary-600 text-white hover:bg-primary-700 active:bg-primary-800',
  // Neutral-900 text on white with a visible border for AA contrast at rest and on hover.
  secondary:
    'bg-white text-neutral-900 border border-neutral-300 hover:border-neutral-400 hover:bg-neutral-50 active:bg-neutral-100',
}

/**
 * Button primitive — primary/secondary/disabled variants meeting WCAG AA contrast (FR-012).
 */
export function Button({ variant = 'primary', className = '', disabled, ...rest }: ButtonProps) {
  const classes = [BASE_CLASSES, VARIANT_CLASSES[variant], className].filter(Boolean).join(' ')
  return <button className={classes} disabled={disabled} {...rest} />
}
