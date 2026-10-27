import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ListingCard } from '@/components/ListingCard'
import { Skeleton } from '@/components/ui/Skeleton'
import { getListings } from '@/services/api'

const AMSTERDAM = { lat: 52.37, lng: 4.9, label: 'Amsterdam' }
const FINDS_RADIUS_KM = 5
const FINDS_COUNT = 3

const STEPS = [
  {
    icon: '🍳',
    title: 'A kitchen has food left',
    body: 'A neighbourhood cook, caterer or community kitchen posts today’s surplus with a reduced price and a pickup window.',
  },
  {
    icon: '🙋',
    title: 'You claim a portion',
    body: 'Browse what’s nearby without signing up. Claim in a couple of taps when something looks good.',
  },
  {
    icon: '🛍️',
    title: 'Pick it up, eat it',
    body: 'Collect it in the window, say hi to your neighbour, and rate the cook so others know who to trust.',
  },
]

const VALUES = [
  {
    title: 'Less food in the bin',
    body: 'Good food that would be thrown out tonight ends up on a plate instead. Every claim is a portion saved.',
  },
  {
    title: 'Neighbours, not strangers',
    body: 'Pickup is local by design. You meet the people who cook around the corner — community kitchens get equal billing.',
  },
  {
    title: 'Fair for everyone',
    body: 'Prices stay well below the menu, fees stay minimal, and cooks still earn something for food that would otherwise be lost.',
  },
  {
    title: 'Trust you can see',
    body: 'Every cook shows their verification status and rating history. Verified status is earned from real buyer ratings.',
  },
]

const FAQ = [
  {
    q: 'Is the food safe?',
    a: 'Every cook passes an automatic business-registration check (an active KvK registration plus a registered kitchen declaration) before they can publish. Listings must follow Dutch food-safety rules.',
  },
  {
    q: 'What does “unverified” mean?',
    a: 'New cooks start unverified and are always labelled that way. After enough good buyer ratings they become verified — and lose it again if ratings drop.',
  },
  {
    q: 'How do I pay?',
    a: 'You pay online when you claim, so pickup is just a quick handover. Your orders stay in your order history.',
  },
  {
    q: 'Do I need an account to look around?',
    a: 'No. Browse everything nearby freely; you only sign in when you claim a portion or start selling.',
  },
  {
    q: 'Who can sell on eyho?',
    a: 'Registered kitchens: restaurants, caterers, home cooks with a registered kitchen, and community or non-profit kitchens. Community kitchens always get equal visibility.',
  },
  {
    q: 'What if I can’t make the pickup?',
    a: 'Each listing has a clear pickup window. Check it before you claim, and see your claim’s details any time in your order history.',
  },
]

/**
 * HomePage — the landing page signed-out visitors see at `/`. Explains the mission and shows a
 * few live finds near Amsterdam (or the visitor's location, only after they opt in).
 */
export function HomePage() {
  const [place, setPlace] = useState(AMSTERDAM)
  const [locating, setLocating] = useState(false)
  const [locateError, setLocateError] = useState<string | null>(null)

  const { data: listings = [], isLoading, isError } = useQuery({
    queryKey: ['listings', place.lat, place.lng, FINDS_RADIUS_KM],
    queryFn: () => getListings(place.lat, place.lng, FINDS_RADIUS_KM),
  })
  const finds = listings.filter((l) => l.portions_remaining > 0).slice(0, FINDS_COUNT)

  function locateMe() {
    if (!navigator.geolocation) {
      setLocateError('Your browser can’t share a location.')
      return
    }
    setLocating(true)
    setLocateError(null)
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        setPlace({ lat: pos.coords.latitude, lng: pos.coords.longitude, label: 'your location' })
        setLocating(false)
      },
      () => {
        setLocateError('Couldn’t get your location — showing Amsterdam.')
        setLocating(false)
      },
    )
  }

  return (
    <main className="overflow-x-clip">
      {/* Hero */}
      <section className="relative isolate">
        <div
          aria-hidden="true"
          className="home-blob absolute -right-24 -top-24 -z-10 h-[28rem] w-[28rem] rounded-full bg-primary-200 opacity-60 blur-3xl"
        />
        <div
          aria-hidden="true"
          className="home-blob home-blob-delay absolute -left-32 top-48 -z-10 h-[22rem] w-[22rem] rounded-full bg-sticker opacity-30 blur-3xl"
        />
        <div className="mx-auto grid max-w-6xl items-center gap-2xl px-md pb-3xl pt-2xl sm:px-lg lg:grid-cols-[1.15fr_1fr] lg:pt-3xl">
          <div className="home-rise">
            <p className="mb-md inline-flex items-center gap-sm rounded-control border border-primary-200 bg-white/70 px-md py-xs text-sm font-semibold text-primary-700">
              <span className="h-2 w-2 animate-pulse rounded-full bg-primary-500 motion-reduce:animate-none" />
              Surplus food from the kitchens around you
            </p>
            <h1 className="mb-md text-[2.75rem] font-extrabold leading-[0.98] text-neutral-900 sm:text-[4.25rem]">
              Good food shouldn’t{' '}
              <span className="relative whitespace-nowrap text-primary-700">
                end in the bin
                <svg
                  aria-hidden="true"
                  viewBox="0 0 300 16"
                  preserveAspectRatio="none"
                  className="absolute -bottom-2 left-0 h-3 w-full text-sticker"
                >
                  <path
                    d="M2 12 C 80 2, 200 2, 298 10"
                    stroke="currentColor"
                    strokeWidth="6"
                    fill="none"
                    strokeLinecap="round"
                  />
                </svg>
              </span>
              .
            </h1>
            <p className="mb-xl max-w-[34rem] text-lg text-neutral-600 sm:text-xl">
              eyho connects neighbourhood kitchens that cooked a little too much with neighbours
              who’d love a good meal for less. Less waste, more community, one portion at a time.
            </p>
            <div className="flex flex-wrap items-center gap-md">
              <Link
                to="/dashboard"
                className="inline-flex min-h-12 items-center rounded-control bg-primary-600 px-xl text-base font-bold text-white no-underline shadow-lg shadow-primary-600/25 transition hover:-translate-y-0.5 hover:bg-primary-700 motion-reduce:transition-none motion-reduce:hover:translate-y-0"
              >
                Start sharing your surplus
              </Link>
              <a
                href="#finds"
                className="text-base font-semibold text-primary-700 underline-offset-4 hover:underline"
              >
                See what’s nearby ↓
              </a>
            </div>
          </div>

          {/* Floating sticker collage */}
          <div aria-hidden="true" className="relative mx-auto hidden h-[26rem] w-full max-w-[28rem] sm:block">
            <div className="home-float absolute left-0 top-6 w-56 rotate-[-6deg] rounded-card border border-neutral-200 bg-white p-md shadow-xl">
              <div className="mb-sm flex h-28 items-center justify-center rounded-[0.8rem] bg-primary-50 text-6xl">🍲</div>
              <p className="font-bold text-neutral-900">Lentil dal</p>
              <p className="text-sm text-neutral-500">Pickup until 20:00</p>
              <span className="price-sticker absolute -right-4 top-24 text-lg">€ 3,50</span>
            </div>
            <div className="home-float home-float-delay absolute right-0 top-32 w-52 rotate-[5deg] rounded-card border border-neutral-200 bg-white p-md shadow-xl">
              <div className="mb-sm flex h-24 items-center justify-center rounded-[0.8rem] bg-[#fff6d6] text-6xl">🥐</div>
              <p className="font-bold text-neutral-900">Bakery bag</p>
              <p className="text-sm text-neutral-500">4 portions left</p>
              <span className="price-sticker absolute -left-4 top-20 text-lg">€ 2,00</span>
            </div>
            <div className="home-float home-float-delay-2 absolute bottom-0 left-16 w-48 rotate-[-2deg] rounded-card border border-neutral-200 bg-white p-md shadow-xl">
              <div className="mb-sm flex h-20 items-center justify-center rounded-[0.8rem] bg-primary-100 text-5xl">🥗</div>
              <p className="font-bold text-neutral-900">Grain bowl</p>
              <span className="price-sticker absolute -right-3 -top-3 text-base">€ 4,00</span>
            </div>
          </div>
        </div>
      </section>

      {/* Mission */}
      <section className="bg-primary-800 text-white">
        <div className="mx-auto max-w-4xl px-md py-3xl text-center sm:px-lg">
          <p className="mb-sm text-sm font-bold uppercase tracking-[0.2em] text-primary-200">Our mission</p>
          <p className="text-2xl font-bold leading-snug sm:text-[2.25rem]">
            Every evening, kitchens throw away food that’s perfectly good. We want that food to feed
            the street instead — at a price anyone can afford, from a cook you can trust.
          </p>
        </div>
      </section>

      {/* How it works */}
      <section className="mx-auto max-w-6xl px-md py-3xl sm:px-lg">
        <h2 className="mb-xl text-center text-3xl font-extrabold text-neutral-900 sm:text-4xl">How it works</h2>
        <ol className="grid gap-lg md:grid-cols-3">
          {STEPS.map((step, i) => (
            <li
              key={step.title}
              className="relative rounded-card border border-neutral-200 bg-white p-xl transition hover:-translate-y-1 hover:shadow-lg motion-reduce:transition-none motion-reduce:hover:translate-y-0"
            >
              <span className="absolute right-lg top-lg text-4xl font-extrabold text-primary-100">{i + 1}</span>
              <span className="mb-md block text-4xl" aria-hidden="true">{step.icon}</span>
              <h3 className="mb-sm text-xl font-bold text-neutral-900">{step.title}</h3>
              <p className="text-neutral-600">{step.body}</p>
            </li>
          ))}
        </ol>
      </section>

      {/* Live finds */}
      <section id="finds" className="scroll-mt-20 bg-primary-50">
        <div className="mx-auto max-w-6xl px-md py-3xl sm:px-lg">
          <div className="mb-xl flex flex-wrap items-end justify-between gap-md">
            <div>
              <h2 className="text-3xl font-extrabold text-neutral-900 sm:text-4xl">Possible finds right now</h2>
              <p className="text-neutral-600">
                Near {place.label}, within {FINDS_RADIUS_KM} km
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-md">
              {place === AMSTERDAM && (
                <button
                  type="button"
                  onClick={locateMe}
                  disabled={locating}
                  className="min-h-11 rounded-control border border-primary-300 bg-white px-lg text-sm font-semibold text-primary-700 hover:bg-primary-100 disabled:opacity-60"
                >
                  {locating ? 'Locating…' : '📍 Use my location'}
                </button>
              )}
              <Link to="/discover" className="text-sm font-semibold text-primary-700">
                Browse all food →
              </Link>
            </div>
          </div>
          {locateError && <p role="status" className="mb-md text-sm text-neutral-700">{locateError}</p>}

          {isLoading && (
            <div className="grid grid-cols-1 gap-lg sm:grid-cols-2 lg:grid-cols-3" aria-busy="true">
              {Array.from({ length: FINDS_COUNT }).map((_, i) => (
                <Skeleton key={i} className="h-80 w-full" label="Loading listing" />
              ))}
            </div>
          )}
          {!isLoading && (isError || finds.length === 0) && (
            <div className="rounded-card border border-dashed border-primary-300 bg-white/60 p-xl text-center">
              <p className="mb-xs text-lg font-bold text-neutral-900">
                {isError ? 'Couldn’t load finds right now.' : 'Nothing up for grabs here just yet.'}
              </p>
              <p className="text-neutral-600">
                Kitchens usually post in the afternoon. Know one with leftovers?{' '}
                <Link to="/dashboard" className="font-semibold text-primary-700">Invite them to share.</Link>
              </p>
            </div>
          )}
          {!isLoading && !isError && finds.length > 0 && (
            <div className="grid grid-cols-1 gap-lg sm:grid-cols-2 lg:grid-cols-3">
              {finds.map((listing) => (
                <ListingCard key={listing.id} listing={listing} href={`/listings/${listing.id}`} />
              ))}
            </div>
          )}
        </div>
      </section>

      {/* Values */}
      <section className="mx-auto max-w-6xl px-md py-3xl sm:px-lg">
        <h2 className="mb-xl text-center text-3xl font-extrabold text-neutral-900 sm:text-4xl">What we stand for</h2>
        <div className="grid gap-lg sm:grid-cols-2">
          {VALUES.map((v) => (
            <div key={v.title} className="flex gap-md rounded-card bg-white p-xl ring-1 ring-neutral-200">
              <span aria-hidden="true" className="mt-1 h-3 w-3 shrink-0 rotate-45 bg-sticker" />
              <div>
                <h3 className="mb-xs text-xl font-bold text-neutral-900">{v.title}</h3>
                <p className="text-neutral-600">{v.body}</p>
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* FAQ */}
      <section className="mx-auto max-w-6xl px-md pb-3xl sm:px-lg">
        <div className="mb-2xl text-center">
          <p className="mb-sm text-sm font-bold uppercase tracking-[0.2em] text-primary-600">Questions</p>
          <h2 className="text-3xl font-extrabold text-neutral-900 sm:text-[3.25rem]">Good to know</h2>
        </div>
        <div className="grid gap-lg md:grid-cols-2">
          {FAQ.map((item) => (
            <div
              key={item.q}
              className="rounded-card border border-neutral-200 bg-white p-xl transition hover:-translate-y-1 hover:shadow-lg motion-reduce:transition-none motion-reduce:hover:translate-y-0 sm:p-2xl"
            >
              <h3 className="mb-md flex items-start gap-md text-xl font-bold text-neutral-900 sm:text-2xl">
                <span aria-hidden="true" className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-primary-100 text-lg text-primary-700">?</span>
                {item.q}
              </h3>
              <p className="text-base text-neutral-600 sm:text-lg">{item.a}</p>
            </div>
          ))}
        </div>
      </section>

      {/* Closing CTA */}
      <section className="px-md pb-3xl sm:px-lg">
        <div className="relative mx-auto max-w-6xl overflow-hidden rounded-[2rem] bg-primary-700 px-xl py-3xl text-center text-white">
          <div aria-hidden="true" className="absolute -right-16 -top-16 h-64 w-64 rounded-full bg-primary-500 opacity-40 blur-2xl" />
          <h2 className="relative mb-md text-3xl font-extrabold sm:text-4xl">Cooked too much tonight?</h2>
          <p className="relative mx-auto mb-xl max-w-[32rem] text-lg text-primary-100">
            Put it up in a minute and let your neighbours enjoy it.
          </p>
          <Link
            to="/dashboard"
            className="relative inline-flex min-h-12 items-center rounded-control bg-sticker px-xl text-base font-extrabold text-sticker-ink no-underline transition hover:-translate-y-0.5 motion-reduce:transition-none motion-reduce:hover:translate-y-0"
          >
            Start sharing your surplus
          </Link>
        </div>
      </section>
    </main>
  )
}
