import { useState } from 'react'
import { Skeleton } from './Skeleton'

export interface ImageWithFallbackProps {
  src: string | null
  alt: string
  className?: string
  /** Sets the CSS aspect-ratio to reserve space and avoid layout shift (FR-009). */
  aspectRatio?: string
}

/**
 * Image component that lazy-loads, reserves space via aspect-ratio to avoid layout
 * shift (FR-009), and shows a branded placeholder (never a broken-image icon) when
 * `src` is null or fails to load (Edge Cases). Shows Skeleton until the image loads.
 */
export function ImageWithFallback({
  src,
  alt,
  className = '',
  aspectRatio = '4 / 3',
}: ImageWithFallbackProps) {
  const [loaded, setLoaded] = useState(false)
  const [errored, setErrored] = useState(false)

  const showPlaceholder = !src || errored
  const wrapperClasses = [
    'relative overflow-hidden rounded-card bg-neutral-100',
    className,
  ]
    .filter(Boolean)
    .join(' ')

  return (
    <div className={wrapperClasses} style={{ aspectRatio }}>
      {!showPlaceholder && !loaded && (
        <Skeleton className="absolute inset-0 h-full w-full rounded-none" label={`Loading ${alt}`} />
      )}

      {showPlaceholder ? (
        <div
          role="img"
          aria-label={alt}
          className="flex h-full w-full flex-col items-center justify-center gap-xs bg-primary-50 text-primary-300"
        >
          {/* Branded placeholder illustration — a simple food bowl glyph, not a broken-image icon. */}
          <svg
            width="48"
            height="48"
            viewBox="0 0 24 24"
            fill="none"
            aria-hidden="true"
            className="text-primary-300"
          >
            <path
              d="M4 12a8 8 0 0016 0H4z"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
            <path
              d="M12 3v3M8 4l1 2M16 4l-1 2"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
            />
          </svg>
          <span className="text-xs font-medium">eyho</span>
        </div>
      ) : (
        <img
          src={src ?? undefined}
          alt={alt}
          loading="lazy"
          onLoad={() => setLoaded(true)}
          onError={() => setErrored(true)}
          className={[
            'h-full w-full object-cover transition-opacity duration-300 motion-reduce:transition-none',
            loaded ? 'opacity-100' : 'opacity-0',
          ].join(' ')}
        />
      )}
    </div>
  )
}
