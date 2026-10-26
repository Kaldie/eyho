/**
 * The buyer's position for discovery and listing detail. v1 uses a fixed Amsterdam point
 * until browser geolocation is wired up; one constant keeps both pages consistent.
 */
export const BUYER_LOCATION = { lat: 52.37, lng: 4.9 } as const
