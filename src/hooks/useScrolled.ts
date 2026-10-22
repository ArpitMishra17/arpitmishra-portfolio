import { useEffect, useState } from 'react'

/** True once the page has scrolled more than `threshold` px from the top. */
export function useScrolled(threshold = 16) {
  const [scrolled, setScrolled] = useState(false)

  useEffect(() => {
    // Setting the same boolean bails out, so this doesn't re-render per frame.
    const onScroll = () => setScrolled(window.scrollY > threshold)
    onScroll()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [threshold])

  return scrolled
}
