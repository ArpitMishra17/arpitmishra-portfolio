import { useEffect } from 'react'
import { useLocation } from 'react-router-dom'

/**
 * React Router navigates to a new path but never scrolls for you, so a link to
 * `/#blog` lands you at the top of the page with the hash ignored. This runs on
 * every location change: scrolls to the top when there's no hash, otherwise to
 * the target element.
 */
export function useHashScroll() {
  const { pathname, hash } = useLocation()

  useEffect(() => {
    if (!hash) {
      window.scrollTo({ top: 0, behavior: 'instant' })
      return
    }

    const id = decodeURIComponent(hash.slice(1))
    // The target may not exist yet on a cold navigation, so retry on the next frame.
    const scrollToTarget = () => {
      const el = document.getElementById(id)
      if (!el) return
      el.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }

    scrollToTarget()
    const raf = requestAnimationFrame(scrollToTarget)
    return () => cancelAnimationFrame(raf)
  }, [pathname, hash])
}
