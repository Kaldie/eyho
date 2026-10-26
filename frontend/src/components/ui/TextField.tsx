import { useId, type InputHTMLAttributes } from 'react'

export interface TextFieldProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'id'> {
  label: string
  /** Inline validation message; announced and linked to the input. */
  error?: string | null
  hint?: string
}

/** Labelled text input with an inline error message, matching the pill-shaped controls. */
export function TextField({ label, error, hint, className = '', ...rest }: TextFieldProps) {
  const id = useId()
  const errorId = `${id}-error`
  const hintId = `${id}-hint`
  return (
    <div className={className}>
      <label htmlFor={id} className="mb-xs block text-sm font-semibold text-neutral-800">
        {label}
      </label>
      <input
        id={id}
        aria-invalid={error ? true : undefined}
        aria-describedby={[error ? errorId : null, hint ? hintId : null].filter(Boolean).join(' ') || undefined}
        className="min-h-[2.75rem] w-full rounded-control border border-neutral-300 bg-white px-md text-base text-neutral-900 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-700"
        {...rest}
      />
      {hint && (
        <p id={hintId} className="mt-xs text-xs text-neutral-600">
          {hint}
        </p>
      )}
      {error && (
        <p id={errorId} role="alert" className="mt-xs text-sm font-medium text-danger">
          {error}
        </p>
      )}
    </div>
  )
}
