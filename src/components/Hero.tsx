import AsciiShader from './AsciiShader'

export default function Hero() {
  return (
    <section
      className="min-h-screen grid grid-cols-1 md:grid-cols-2 items-center border-b"
      style={{ borderColor: 'var(--border)' }}
    >
      <div className="relative overflow-hidden w-full h-full hidden md:flex items-center justify-center">
        <AsciiShader />
      </div>

      <div className="max-w-[530px] px-8 py-10 md:py-0">
        <div className="flex flex-col gap-5">
          <div className="text-[13px] tracking-wider" style={{ color: 'var(--dim)' }}>
            {'// hello world'}
          </div>

          <h1
            className="text-[52px] font-normal tracking-wider leading-[1.15]"
            style={{ fontFamily: "'Geist Pixel', 'Geist Mono', monospace" }}
          >
            Arpit
            <br />
            Mishra
          </h1>

          <div className="text-[16px] tracking-wider" style={{ color: 'var(--accent)' }}>
            software engineer
          </div>

          <p className="leading-[1.8] text-[14px] max-w-[440px]" style={{ color: 'var(--dim)' }}>
            I build software that works well and feels right. Full-stack web
            applications, backend systems, and ai applications.
            <br /><br />
            I care about clean architecture, performance, and writing code that other people
            can actually read.
          </p>

          <div className="flex items-center gap-2 text-[13px]" style={{ color: 'var(--green)' }}>
            <span
              className="w-1.5 h-1.5 rounded-full"
              style={{
                background: 'var(--green)',
                boxShadow: '0 0 8px var(--green)',
                animation: 'pulse 2s ease-in-out infinite',
              }}
            />
            available for work
          </div>

          <div className="flex gap-2.5 items-center">
            <a
              href="#"
              className="inline-flex items-center gap-1.5 px-5 py-2.5 text-[13px] tracking-wider transition-all hover:no-underline"
              style={{
                background: 'none',
                color: 'var(--fg)',
                border: '1px solid var(--border)',
              }}
              onMouseEnter={e => {
                e.currentTarget.style.borderColor = 'var(--accent)'
                e.currentTarget.style.color = 'var(--accent)'
              }}
              onMouseLeave={e => {
                e.currentTarget.style.borderColor = 'var(--border)'
                e.currentTarget.style.color = 'var(--fg)'
              }}
            >
              <svg viewBox="0 0 24 24" className="w-3.5 h-3.5 fill-current">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6zM6 20V4h7v5h5v11H6z" />
              </svg>
              Resume
            </a>
          </div>

          <div className="flex gap-4 text-[13px]">
            {[
              { label: 'github', href: 'https://github.com/ArpitMishra17' },
              { label: 'linkedin', href: 'https://www.linkedin.com/in/arpit-mishra-7a1823277/' },
              { label: 'twitter', href: 'https://x.com/0xarpitm' },
              { label: 'email', href: 'mailto:mishrarpit3117@gmail.com' },
            ].map(link => (
              <a
                key={link.label}
                href={link.href}
                target={link.label !== 'email' ? '_blank' : undefined}
                rel={link.label !== 'email' ? 'noopener noreferrer' : undefined}
                className="transition-colors hover:no-underline"
                style={{ color: 'var(--dim)' }}
                onMouseEnter={e => (e.currentTarget.style.color = 'var(--accent)')}
                onMouseLeave={e => (e.currentTarget.style.color = 'var(--dim)')}
              >
                {link.label}
              </a>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}
