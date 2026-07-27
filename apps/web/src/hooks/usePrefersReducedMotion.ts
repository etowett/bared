import { useSyncExternalStore } from 'react'

const QUERY = '(prefers-reduced-motion: reduce)'

const NOOP = () => () => {}

function subscribe(onStoreChange: () => void) {
  if (typeof window === 'undefined' || !window.matchMedia) return NOOP()

  const mediaQuery = window.matchMedia(QUERY)
  mediaQuery.addEventListener('change', onStoreChange)
  return () => mediaQuery.removeEventListener('change', onStoreChange)
}

function getSnapshot(): boolean {
  if (typeof window === 'undefined' || !window.matchMedia) return false
  return window.matchMedia(QUERY).matches
}

/**
 * Whether the viewer has asked the OS to reduce motion.
 *
 * CSS covers most of this through Tailwind's `motion-safe:` variant; this hook
 * is for what only JavaScript can reach — chiefly `scrollIntoView`, whose
 * `behavior: 'smooth'` ignores the media query entirely.
 */
export function usePrefersReducedMotion(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, () => false)
}
