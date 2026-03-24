import { useState } from 'react'
import ScrollReveal from './ScrollReveal'

const experiences = [
  {
    company: 'Zed',
    role: 'Open Source Contributor',
    period: 'Mar 2026 — Present',
    current: true,
    desc: 'Accepted into the Zed Guild, where I collaborate with the core team to tackle real issues in an editor developed openly.<br/>Contributing to globally used developer features while strengthening my open-source contribution experience.',
  },
  {
    company: 'airevo LABS',
    role: 'Software Engineering Intern',
    period: 'Jan 2026 — Present',
    current: true,
    desc: 'Working as founding engineer on the core ATS platform.<br/>Architected the entire Whatsapp integration for parallel whatsapp and email communication.<br/>Integrated a custom knowledge graph system into the ATS platform.',
  },
  {
    company: 'Aeternik',
    role: 'Software Engineering Intern',
    period: 'Sep 2025 — Jan 2026',
    current: false,
    desc: 'Developed an advanced RAG system for production with a modular framework.<br/>Architected and containerized a microservices based AI backend using Go, Python and Docker.<br/>Minimized latency and hallucinations by implementing caching and feedback loops.',
  },
  {
    company: 'NALCO',
    role: 'Software Engineering Intern',
    period: 'June 2025',
    current: false,
    desc: 'Developed an internal RAG tool using FastAPI and PostgreSQL for document QA.<br/>Implemented PDF ingestion, chunking, and embedding with sentence-transformers for vector search.<br/>Created a full-stack app with a FastAPI backend and HTML/CSS/JS frontend for user interaction.',
  },
  {
    company: 'K J Somaiya College of Engineering',
    role: 'Machine Learning Research Intern',
    period: 'Feb 2025 — Present',
    current: false,
    desc: 'Managing data pipelines for agricultural datasets, ensuring data quality for ML applications.<br/>Building and optimizing predictive ML models for accuracy and clarity.',
  },
  {
    company: 'SMLRA',
    role: 'Committee Member',
    period: 'Aug 2024 — May 2025',
    current: false,
    desc: 'Conducted research on legal document summarization to explore innovative applications of technology.<br/>Contributed to event management by preparing presentations and assisting in hosting activities.<br/>Created engaging social media stories such as Fact Friday and ML Monday to share knowledge and drive community interaction.',
  },
  {
    company: 'Secuodsoft Technologies',
    role: 'Python Developer Intern',
    period: 'Dec 2024',
    current: false,
    desc: 'Developed backend features using Django and MySQL.<br/>Implemented frontend components using JS and Bootstrap.<br/>Worked with static and media files for handling assets and uploads.',
  },
]

function renderBulletPoints(htmlLikeText: string) {
  const points = htmlLikeText.split(/<br\s*\/?>/gi).filter(p => p.trim())
  return (
    <ul className="flex flex-col gap-2 list-none m-0 p-0">
      {points.map((point, i) => (
        <li key={i} className="flex items-start gap-2">
          <span className="mt-[7px] shrink-0 w-[5px] h-[5px] rounded-full" style={{ background: 'var(--accent)' }} />
          <span>{point.trim()}</span>
        </li>
      ))}
    </ul>
  )
}

export default function Experience() {
  const [expandedIndex, setExpandedIndex] = useState<number | null>(null)

  const toggle = (i: number) => {
    setExpandedIndex(prev => (prev === i ? null : i))
  }

  return (
    <section className="py-20 border-b" style={{ borderColor: 'var(--border)' }} id="experience">
      <div className="max-w-[1060px] mx-auto px-5 md:px-8">
        <div className="mb-10">
          <h2
            className="text-[32px] tracking-[3px] uppercase font-light"
            style={{ color: 'var(--fg)' }}
          >
            Experience
          </h2>
          <span className="text-[12px] mt-1 inline-block" style={{ color: 'var(--accent)' }}>7 roles</span>
        </div>

        <ScrollReveal stagger>
          <div
            className="flex flex-col"
            style={{ gap: '1px', background: 'var(--border)', border: '1px solid var(--border)' }}
          >
            {experiences.map((exp, i) => {
              const isOpen = expandedIndex === i
              return (
                <div
                  key={i}
                  className="reveal sweep-hover flex flex-col gap-3 p-6 px-7 relative cursor-pointer select-none"
                  style={{ background: 'var(--bg2)' }}
                  onClick={() => toggle(i)}
                  onMouseEnter={e =>
                    (e.currentTarget.style.background =
                      'color-mix(in srgb, var(--bg2) 92%, var(--accent) 8%)')
                  }
                  onMouseLeave={e => (e.currentTarget.style.background = 'var(--bg2)')}
                >
                  {exp.current && (
                    <div
                      className="absolute top-0 left-0 w-[3px] h-full"
                      style={{ background: 'var(--accent)' }}
                    />
                  )}
                  <div className="flex items-baseline gap-x-3">
                    <div className="flex flex-wrap items-baseline gap-x-3 flex-1">
                      <span
                        className="text-[22px] md:text-[26px] tracking-wider"
                        style={{
                          fontFamily: "'Geist Pixel', 'Geist Mono', monospace",
                          color: 'var(--fg)',
                        }}
                      >
                        {exp.role}
                      </span>
                      <span
                        className="text-[15px] md:text-[18px] italic"
                        style={{ color: 'var(--accent)' }}
                      >
                        @ {exp.company}
                      </span>
                    </div>
                    <svg
                      width="20"
                      height="20"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="1.5"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      className="shrink-0 transition-transform duration-300"
                      style={{
                        color: 'var(--dim)',
                        transform: isOpen ? 'rotate(180deg)' : 'rotate(0deg)',
                      }}
                    >
                      <polyline points="6 9 12 15 18 9" />
                    </svg>
                  </div>
                  <div
                    className="text-[12px] tracking-[2px] uppercase"
                    style={{ color: 'var(--dim)' }}
                  >
                    {exp.period}
                  </div>
                  <div
                    className="overflow-hidden transition-all duration-300 ease-in-out"
                    style={{
                      maxHeight: isOpen ? '500px' : '0px',
                      opacity: isOpen ? 1 : 0,
                    }}
                  >
                    <div
                      className="text-[15px] leading-[1.7] mt-1"
                      style={{ color: 'var(--dim)' }}
                    >
                      {renderBulletPoints(exp.desc)}
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        </ScrollReveal>
      </div>
    </section>
  )
}
