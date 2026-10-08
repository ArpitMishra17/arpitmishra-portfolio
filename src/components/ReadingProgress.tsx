import { useEffect, useRef } from 'react'

export function ReadingProgress() {
  const barRef = useRef<HTMLDivElement>(null)

  // Writes go straight to the DOM via transform. No React state means no
  // re-render on every scroll frame.
  useEffect(() => {
    const bar = barRef.current
    if (!bar) return

    let frame = 0
    const update = () => {
      frame = 0
      const docHeight = document.documentElement.scrollHeight - window.innerHeight
      const p = docHeight > 0 ? Math.min(1, Math.max(0, window.scrollY / docHeight)) : 0
      bar.style.transform = `scaleX(${p})`
    }
    const onScroll = () => {
      if (frame) return
      frame = requestAnimationFrame(update)
    }

    update()
    window.addEventListener('scroll', onScroll, { passive: true })
    window.addEventListener('resize', onScroll)
    return () => {
      if (frame) cancelAnimationFrame(frame)
      window.removeEventListener('scroll', onScroll)
      window.removeEventListener('resize', onScroll)
    }
  }, [])

  return (
    <div
      className="fixed top-0 left-0 right-0 h-[2px] z-[60] pointer-events-none"
      style={{ background: 'transparent' }}
      aria-hidden="true"
    >
      {/* scaleX, not width: no layout on every scroll frame.
          No transition — the bar must track the scroll exactly, not lag it. */}
      <div
        ref={barRef}
        className="h-full w-full bar-scale"
        style={{ background: 'var(--accent)', transform: 'scaleX(0)' }}
      />
    </div>
  )
}
