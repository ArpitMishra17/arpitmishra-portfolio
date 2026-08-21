import { useEffect, useState } from 'react'
import type { TocItem } from '../../lib/utils'

interface Props {
  items: TocItem[]
  className?: string
  showHeader?: boolean
}

export function TableOfContents({ items, className, showHeader = true }: Props) {
  const [activeSlug, setActiveSlug] = useState<string | null>(null)

  useEffect(() => {
    if (items.length === 0) return
    const headings = items.map(i => document.getElementById(i.slug)).filter((el): el is HTMLElement => !!el)
    if (headings.length === 0) return
    const obs = new IntersectionObserver(
      entries => {
        const visible = entries.filter(e => e.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)
        if (visible.length > 0) setActiveSlug(visible[0].target.id)
      },
      { rootMargin: '-80px 0px -70% 0px', threshold: 0 },
    )
    for (const h of headings) obs.observe(h)
    return () => obs.disconnect()
  }, [items])

  if (items.length === 0) return null

  return (
    <nav aria-label="On this page" className={className}>
      {showHeader && <p className="text-[11px] tracking-[1.5px] uppercase mb-3" style={{ color: 'var(--dim)' }}>On this page</p>}
      <ul className="list-none m-0 p-0 flex flex-col gap-1">
        {items.map(item => {
          const isActive = activeSlug === item.slug
          const indentClass = item.level === 4 ? 'ml-8' : item.level === 3 ? 'ml-4' : ''
          return (
            <li key={item.slug} className={indentClass}>
              <a
                href={`#${item.slug}`}
                className="flex items-center gap-2 py-1 text-[13px] leading-snug transition-colors"
                style={{
                  color: isActive ? 'var(--accent)' : 'var(--dim)',
                  textDecoration: 'none',
                }}
                onMouseEnter={e => {
                  if (!isActive) e.currentTarget.style.color = 'var(--fg)'
                }}
                onMouseLeave={e => {
                  if (!isActive) e.currentTarget.style.color = 'var(--dim)'
                }}
              >
                <span className="shrink-0 text-[10px]" style={{ color: isActive ? 'var(--accent)' : 'var(--border)' }}>
                  →
                </span>
                <span>{item.text}</span>
              </a>
            </li>
          )
        })}
      </ul>
    </nav>
  )
}
