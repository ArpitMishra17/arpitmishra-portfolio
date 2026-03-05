const techGroups = [
  {
    title: 'Languages',
    items: ['Python', 'C++', 'JavaScript', 'TypeScript', 'Go', 'Solidity'],
  },
  {
    title: 'Frameworks',
    items: ['React', 'Node.js', 'Express', 'Next.js', 'FastAPI', 'Django', 'Tailwind CSS'],
  },
  {
    title: 'Developer Tools',
    items: ['Git', 'GitHub', 'VSCode', 'Jupyter', 'Docker', 'Hardhat', 'LangChain'],
  },
  {
    title: 'Data',
    items: ['PostgreSQL', 'Weaviate', 'Redis', 'MongoDB', 'pgvector'],
  },
]

export default function Technologies() {
  return (
    <section className="py-20 border-b" style={{ borderColor: 'var(--border)' }} id="tech">
      <div className="max-w-[1060px] mx-auto px-5 md:px-8">
        <div
          className="text-[11px] tracking-[2.5px] uppercase mb-6 flex justify-between items-center"
          style={{ color: 'var(--dim)' }}
        >
          <span>Technologies</span>
        </div>

        <div
          className="grid grid-cols-2 md:grid-cols-4"
          style={{ gap: '1px', background: 'var(--border)', border: '1px solid var(--border)' }}
        >
          {techGroups.map((group, i) => (
            <div
              key={i}
              className="p-5 transition-colors"
              style={{ background: 'var(--bg2)' }}
              onMouseEnter={e =>
                (e.currentTarget.style.background =
                  'color-mix(in srgb, var(--bg2) 92%, var(--accent) 8%)')
              }
              onMouseLeave={e => (e.currentTarget.style.background = 'var(--bg2)')}
            >
              <div
                className="text-[11px] tracking-[1.5px] uppercase mb-3"
                style={{ color: 'var(--dim)' }}
              >
                {group.title}
              </div>
              <div className="flex flex-col gap-1.5">
                {group.items.map(item => (
                  <div
                    key={item}
                    className="text-[14px] py-0.5 cursor-default transition-colors"
                    style={{ color: 'var(--fg)' }}
                    onMouseEnter={e => (e.currentTarget.style.color = 'var(--accent)')}
                    onMouseLeave={e => (e.currentTarget.style.color = 'var(--fg)')}
                  >
                    <span
                      className="transition-colors"
                      style={{ color: 'var(--border)' }}
                    >
                      →{' '}
                    </span>
                    {item}
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
