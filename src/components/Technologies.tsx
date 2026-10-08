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
        <div className="mb-10">
          <h2
            className="text-[clamp(24px,3vw,32px)] tracking-[3px] uppercase font-medium"
            style={{ color: 'var(--fg)' }}
          >
            Technologies
          </h2>
        </div>

        <div
          className="grid grid-cols-2 md:grid-cols-4"
          style={{ gap: '1px', background: 'var(--border)', border: '1px solid var(--border)' }}
        >
          {techGroups.map((group, i) => (
            <div
              key={i}
              className="p-5 hover-cell"
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
                    className="hover-accent text-[14px] py-0.5 cursor-default"
                  >
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
