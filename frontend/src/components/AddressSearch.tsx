import { useEffect, useId, useRef, useState } from 'react'

export interface AddressSelection {
  text: string
  lat: number
  lng: number
}

interface Suggestion {
  id: string
  label: string
}

interface AddressSearchProps {
  label: string
  value: AddressSelection | null
  onChange: (value: AddressSelection | null) => void
  error?: string | null
  /** Overridable for tests. */
  baseUrl?: string
}

const PDOK = 'https://api.pdok.nl/bzk/locatieserver/search/v3_1'

/** PDOK returns centroids as WKT: "POINT(4.89 52.37)" (lng lat). */
export function parsePoint(wkt: string): { lat: number; lng: number } | null {
  const m = /POINT\(\s*(-?[\d.]+)\s+(-?[\d.]+)\s*\)/.exec(wkt)
  return m ? { lng: Number(m[1]), lat: Number(m[2]) } : null
}

/**
 * Address autocomplete on the PDOK Locatieserver (suggest, then lookup; free, no key).
 * Calls `onChange` with address text and lat/lng once a suggestion is chosen, and with null
 * whenever the person edits the text again, so a stale location is never submitted.
 */
export function AddressSearch({ label, value, onChange, error, baseUrl = PDOK }: AddressSearchProps) {
  const inputId = useId()
  const listId = useId()
  const [query, setQuery] = useState(value?.text ?? '')
  const [suggestions, setSuggestions] = useState<Suggestion[]>([])
  const [open, setOpen] = useState(false)
  const [lookupFailed, setLookupFailed] = useState(false)
  const skipNext = useRef(false)

  useEffect(() => {
    if (skipNext.current) {
      skipNext.current = false
      return
    }
    const q = query.trim()
    if (q.length < 3) {
      setSuggestions([])
      return
    }
    const controller = new AbortController()
    const timer = window.setTimeout(async () => {
      try {
        const params = new URLSearchParams({ q, rows: '5', fq: 'type:adres' })
        const res = await fetch(`${baseUrl}/suggest?${params}`, { signal: controller.signal })
        if (!res.ok) throw new Error(String(res.status))
        const body = (await res.json()) as {
          response?: { docs?: { id: string; weergavenaam: string }[] }
        }
        setSuggestions(
          (body.response?.docs ?? []).map((d) => ({ id: d.id, label: d.weergavenaam })),
        )
        setOpen(true)
        setLookupFailed(false)
      } catch (err) {
        if ((err as Error).name !== 'AbortError') setLookupFailed(true)
      }
    }, 250)
    return () => {
      controller.abort()
      window.clearTimeout(timer)
    }
  }, [query, baseUrl])

  async function choose(s: Suggestion) {
    setOpen(false)
    try {
      const res = await fetch(`${baseUrl}/lookup?${new URLSearchParams({ id: s.id, fl: 'id,weergavenaam,centroide_ll' })}`)
      if (!res.ok) throw new Error(String(res.status))
      const body = (await res.json()) as {
        response?: { docs?: { weergavenaam: string; centroide_ll: string }[] }
      }
      const doc = body.response?.docs?.[0]
      const point = doc ? parsePoint(doc.centroide_ll) : null
      if (!doc || !point) throw new Error('no location')
      skipNext.current = true
      setQuery(doc.weergavenaam)
      setLookupFailed(false)
      onChange({ text: doc.weergavenaam, ...point })
    } catch {
      setLookupFailed(true)
    }
  }

  return (
    <div className="relative">
      <label htmlFor={inputId} className="mb-xs block text-sm font-semibold text-neutral-800">
        {label}
      </label>
      <input
        id={inputId}
        type="text"
        role="combobox"
        aria-expanded={open && suggestions.length > 0}
        aria-controls={listId}
        aria-autocomplete="list"
        aria-invalid={error ? true : undefined}
        autoComplete="street-address"
        value={query}
        onChange={(e) => {
          setQuery(e.target.value)
          if (value) onChange(null)
        }}
        onFocus={() => suggestions.length > 0 && setOpen(true)}
        className="min-h-[2.75rem] w-full rounded-control border border-neutral-300 bg-white px-md text-base text-neutral-900"
      />
      {open && suggestions.length > 0 && (
        <ul
          id={listId}
          role="listbox"
          className="absolute z-20 mt-xs w-full overflow-hidden rounded-card border border-neutral-200 bg-white shadow-lg"
        >
          {suggestions.map((s) => (
            <li key={s.id} role="option" aria-selected={false}>
              <button
                type="button"
                onClick={() => void choose(s)}
                className="w-full px-md py-sm text-left text-sm text-neutral-800 hover:bg-neutral-100"
              >
                {s.label}
              </button>
            </li>
          ))}
        </ul>
      )}
      {value && <p className="mt-xs text-xs text-neutral-600">Address found. Pickup point saved.</p>}
      {lookupFailed && (
        <p role="alert" className="mt-xs text-sm text-neutral-700">
          We couldn&apos;t look up that address. Check your connection and try again.
        </p>
      )}
      {error && (
        <p role="alert" className="mt-xs text-sm font-medium text-danger">
          {error}
        </p>
      )}
    </div>
  )
}
