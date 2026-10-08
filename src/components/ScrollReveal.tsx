import { useEffect, useRef, type ReactNode } from 'react'

interface ScrollRevealProps {
  children: ReactNode
  className?: string
  stagger?: boolean
}

export default function ScrollReveal({ children, className = '', stagger = false }: ScrollRevealProps) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const el = ref.current
    if (!el) return

    // Reveal immediately where IntersectionObserver is unavailable or the user
    // has asked for reduced motion — hidden content that never appears is worse
    // than no animation at all.
    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    if (reduceMotion || typeof IntersectionObserver === 'undefined') {
      el.querySelectorAll('.reveal').forEach(r => r.classList.add('visible'))
      return
    }

    const observer = new IntersectionObserver(
      entries => {
        entries.forEach(entry => {
          if (!entry.isIntersecting) return
          entry.target.classList.add('visible')
          // Reveal is a one-shot entrance; stop observing once fired.
          observer.unobserve(entry.target)
        })
      },
      // Fires slightly before the element is fully on screen, so content is
      // already settled by the time it's readable.
      { rootMargin: '0px 0px -10% 0px', threshold: 0.05 }
    )

    const reveals = el.querySelectorAll('.reveal')
    reveals.forEach(r => observer.observe(r))

    return () => observer.disconnect()
  }, [])

  return (
    <div ref={ref} className={`${stagger ? 'stagger' : ''} ${className}`}>
      {children}
    </div>
  )
}
