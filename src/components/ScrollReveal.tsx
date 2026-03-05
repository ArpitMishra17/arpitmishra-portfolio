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

    const observer = new IntersectionObserver(
      entries => {
        entries.forEach(entry => {
          if (entry.isIntersecting) {
            entry.target.classList.add('visible')
          }
        })
      },
      { threshold: 0.1 }
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
