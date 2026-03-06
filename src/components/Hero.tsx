import AsciiShader from './AsciiShader'

export default function Hero() {
  return (
    <section
      id="hero"
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

          <div className="flex gap-8 items-center">
            {[
              {
                label: 'github',
                href: 'https://github.com/ArpitMishra17',
                icon: (
                  <svg viewBox="0 0 24 24" className="w-[18px] h-[18px] fill-current">
                    <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z" />
                  </svg>
                ),
              },
              {
                label: 'linkedin',
                href: 'https://www.linkedin.com/in/arpit-mishra-7a1823277/',
                icon: (
                  <svg viewBox="0 0 24 24" className="w-[18px] h-[18px] fill-current">
                    <path d="M20.447 20.452h-3.554v-5.569c0-1.328-.027-3.037-1.852-3.037-1.853 0-2.136 1.445-2.136 2.939v5.667H9.351V9h3.414v1.561h.046c.477-.9 1.637-1.85 3.37-1.85 3.601 0 4.267 2.37 4.267 5.455v6.286zM5.337 7.433a2.062 2.062 0 0 1-2.063-2.065 2.064 2.064 0 1 1 2.063 2.065zm1.782 13.019H3.555V9h3.564v11.452zM22.225 0H1.771C.792 0 0 .774 0 1.729v20.542C0 23.227.792 24 1.771 24h20.451C23.2 24 24 23.227 24 22.271V1.729C24 .774 23.2 0 22.222 0h.003z" />
                  </svg>
                ),
              },
              {
                label: 'x',
                href: 'https://x.com/0xarpitm',
                icon: (
                  <svg viewBox="0 0 24 24" className="w-[18px] h-[18px] fill-current">
                    <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
                  </svg>
                ),
              },
              {
                label: 'email',
                href: 'mailto:mishrarpit3117@gmail.com',
                icon: (
                  <svg viewBox="0 0 24 24" className="w-[18px] h-[18px] fill-none stroke-current" strokeWidth="1.5">
                    <path d="M21.75 6.75v10.5a2.25 2.25 0 0 1-2.25 2.25h-15a2.25 2.25 0 0 1-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25m19.5 0-8.953 5.468a1.5 1.5 0 0 1-1.594 0L2.25 6.75" strokeLinecap="round" strokeLinejoin="round" />
                  </svg>
                ),
              },
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
                {link.icon}
              </a>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}
