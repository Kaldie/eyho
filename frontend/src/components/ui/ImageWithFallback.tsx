import { useState } from 'react'
import { Skeleton } from './Skeleton'

export interface ImageWithFallbackProps {
  src: string | null
  alt: string
  className?: string
  /** Sets the CSS aspect-ratio to reserve space and avoid layout shift (FR-009). */
  aspectRatio?: string
}

// Food-hued tints for placeholder tiles, picked deterministically per dish so the same
// listing always gets the same tile and a grid of placeholders doesn't look uniform.
const PLACEHOLDER_TINTS = [
  { bg: '#e7efe2', fg: '#3f5a35' }, // herb
  { bg: '#f6e7da', fg: '#7a4a25' }, // crust
  { bg: '#f3e3e1', fg: '#7d3a33' }, // tomato
  { bg: '#efe9d4', fg: '#5f5320' }, // grain
  { bg: '#e3ebee', fg: '#30505c' }, // rinse
]

function tintFor(text: string) {
  let hash = 0
  for (let i = 0; i < text.length; i++) hash = (hash * 31 + text.charCodeAt(i)) >>> 0
  return PLACEHOLDER_TINTS[hash % PLACEHOLDER_TINTS.length]
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
  const wrapperClasses = ['relative overflow-hidden rounded-card bg-neutral-100', className]
    .filter(Boolean)
    .join(' ')
  const tint = tintFor(alt)
  const initial = alt.trim().charAt(0).toUpperCase() || 'e'

  return (
    <div className={wrapperClasses} style={{ aspectRatio }}>
      {!showPlaceholder && !loaded && (
        <Skeleton className="absolute inset-0 h-full w-full rounded-none" label={`Loading ${alt}`} />
      )}

      {showPlaceholder ? (
        <div
          role="img"
          aria-label={alt}
          className="relative flex h-full w-full items-center justify-center @container"
          style={{ background: tint.bg, color: tint.fg }}
        >
          {/* Branded placeholder: the dish's initial on a food-hued tile, never a broken-image icon. */}
          <span aria-hidden="true" className="text-[clamp(1.5rem,32cqw,4.5rem)] font-extrabold leading-none opacity-80">
            {initial}
          </span>
          <span className="absolute bottom-[6%] left-[6%] text-[clamp(0.5rem,5cqw,0.75rem)] font-bold">eyho</span>
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
