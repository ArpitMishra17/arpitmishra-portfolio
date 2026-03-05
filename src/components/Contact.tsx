const links = [
  { label: 'mishrarpit3117@gmail.com', href: 'mailto:mishrarpit3117@gmail.com' },
  { label: 'github.com/ArpitMishra17', href: 'https://github.com/ArpitMishra17' },
  { label: 'linkedin.com/in/arpit-mishra', href: 'https://www.linkedin.com/in/arpit-mishra-7a1823277/' },
  { label: '@0xarpitm', href: 'https://x.com/0xarpitm' },
]

export default function Contact() {
  return (
    <div>
      <div
        className="text-[11px] tracking-[2.5px] uppercase mb-6 flex justify-between items-center"
        style={{ color: 'var(--dim)' }}
      >
        <span>Contact</span>
      </div>

      <div className="flex flex-col md:flex-row justify-between items-center py-5 gap-3 md:gap-0">
        <div className="flex flex-wrap justify-center md:justify-start gap-6">
          {links.map((link, i) => (
            <a
              key={i}
              href={link.href}
              className="text-[13px] transition-colors hover:no-underline"
              style={{ color: 'var(--dim)' }}
              onMouseEnter={e => (e.currentTarget.style.color = 'var(--accent)')}
              onMouseLeave={e => (e.currentTarget.style.color = 'var(--dim)')}
            >
              <span className="text-[12px] mr-1.5" style={{ color: 'var(--dim)' }}>
                →
              </span>
              {link.label}
            </a>
          ))}
        </div>
        <div className="text-[12px] tracking-wider opacity-50" style={{ color: 'var(--dim)' }}>
          last updated 2026.03.05
        </div>
      </div>
    </div>
  )
}
