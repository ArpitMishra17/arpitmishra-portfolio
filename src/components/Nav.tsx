interface NavProps {
  theme: 'dark' | 'light'
  onToggleTheme: () => void
}

export default function Nav({ theme, onToggleTheme }: NavProps) {
  return (
    <nav
      className="fixed top-0 left-0 right-0 z-[1000] flex justify-between items-center backdrop-blur-[12px] border-b transition-colors"
      style={{
        background: 'color-mix(in srgb, var(--bg) 85%, transparent)',
        borderColor: 'var(--border)',
        fontSize: '13px',
        padding: '10px 32px',
      }}
    >
      <div className="flex items-center" style={{ gap: '6px' }}>
        <a
          href="#hero"
          className="font-normal hover:no-underline"
          style={{ color: 'var(--fg)', fontFamily: "'Geist Pixel', monospace", letterSpacing: '1px', textDecoration: 'none' }}
        >
          arpit_mishra
        </a>
        <span style={{ color: 'var(--border)' }}>/</span>
        <div className="hidden md:flex" style={{ gap: '2px' }}>
          {['experience', 'projects', 'tech', 'contact'].map(id => (
            <a
              key={id}
              href={`#${id}`}
              className="transition-colors hover:no-underline"
              style={{ color: 'var(--dim)', padding: '4px 10px' }}
              onMouseEnter={e => (e.currentTarget.style.color = 'var(--accent)')}
              onMouseLeave={e => (e.currentTarget.style.color = 'var(--dim)')}
            >
              {id}
            </a>
          ))}
        </div>
      </div>
      <div className="flex items-center" style={{ gap: '10px' }}>
        <button
          onClick={onToggleTheme}
          className="cursor-pointer"
          aria-label={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}
          style={{
            position: 'relative',
            width: '56px',
            height: '28px',
            borderRadius: '9999px',
            border: 'none',
            background: theme === 'dark' ? 'var(--bg)' : 'var(--border)',
            padding: '3px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            transition: 'background 0.3s ease',
          }}
        >
          {/* Sun icon */}
          <span
            style={{
              width: '22px',
              height: '22px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              zIndex: 1,
              color: theme === 'light' ? '#fff' : 'var(--dim)',
              transition: 'color 0.3s ease',
            }}
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="5" />
              <line x1="12" y1="1" x2="12" y2="3" />
              <line x1="12" y1="21" x2="12" y2="23" />
              <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
              <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
              <line x1="1" y1="12" x2="3" y2="12" />
              <line x1="21" y1="12" x2="23" y2="12" />
              <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
              <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
            </svg>
          </span>
          {/* Moon icon */}
          <span
            style={{
              width: '22px',
              height: '22px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              zIndex: 1,
              color: theme === 'dark' ? '#fff' : 'var(--dim)',
              transition: 'color 0.3s ease',
            }}
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
            </svg>
          </span>
          {/* Sliding circle */}
          <span
            style={{
              position: 'absolute',
              top: '3px',
              left: theme === 'light' ? '3px' : '31px',
              width: '22px',
              height: '22px',
              borderRadius: '9999px',
              background: 'var(--accent)',
              transition: 'left 0.3s ease',
            }}
          />
        </button>
      </div>
    </nav>
  )
}
