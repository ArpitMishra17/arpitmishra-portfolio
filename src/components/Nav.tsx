interface NavProps {
  onToggleTheme: () => void
}

export default function Nav({ onToggleTheme }: NavProps) {
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
        <span
          className="font-normal"
          style={{ color: 'var(--fg)', fontFamily: "'Geist Pixel', monospace", letterSpacing: '1px' }}
        >
          arpit_mishra
        </span>
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
          className="cursor-pointer transition-all"
          style={{
            background: 'none',
            border: '1px solid var(--border)',
            color: 'var(--dim)',
            fontFamily: 'inherit',
            fontSize: '13px',
            padding: '4px 14px',
          }}
          onMouseEnter={e => {
            e.currentTarget.style.borderColor = 'var(--accent)'
            e.currentTarget.style.color = 'var(--accent)'
          }}
          onMouseLeave={e => {
            e.currentTarget.style.borderColor = 'var(--border)'
            e.currentTarget.style.color = 'var(--dim)'
          }}
        >
          theme
        </button>
      </div>
    </nav>
  )
}
