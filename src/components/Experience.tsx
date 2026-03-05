import { Fragment } from 'react'
import ScrollReveal from './ScrollReveal'

const experiences = [
  {
    company: 'Zed',
    role: 'Guild Member',
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

function renderLineBreaks(htmlLikeText: string) {
  const parts = htmlLikeText.split(/<br\s*\/?>/gi)
  return parts.map((part, i) => (
    <Fragment key={i}>
      {part}
      {i < parts.length - 1 && <br />}
    </Fragment>
  ))
}

export default function Experience() {
  return (
    <section className="py-20 border-b" style={{ borderColor: 'var(--border)' }} id="experience">
      <div className="max-w-[1060px] mx-auto px-5 md:px-8">
        <div
          className="text-[11px] tracking-[2.5px] uppercase mb-6 flex justify-between items-center"
          style={{ color: 'var(--dim)' }}
        >
          <span>Experience</span>
          <span className="text-[12px]" style={{ color: 'var(--accent)' }}>7 roles</span>
        </div>

        <ScrollReveal stagger>
          <div
            className="flex flex-col"
            style={{ gap: '1px', background: 'var(--border)', border: '1px solid var(--border)' }}
          >
            {experiences.map((exp, i) => (
              <div
                key={i}
                className="reveal sweep-hover grid grid-cols-1 md:grid-cols-[160px_1fr] gap-5 p-5 px-6 items-start relative"
                style={{ background: 'var(--bg2)' }}
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
                <div className="text-[14px]" style={{ color: 'var(--fg)' }}>
                  {exp.company}
                </div>
                <div className="flex flex-col gap-1">
                  <div className="text-[16px]" style={{ color: 'var(--accent)' }}>
                    {exp.role}
                  </div>
                  <div
                    className="text-[13px] tracking-wider pt-0.5"
                    style={{ color: 'var(--dim)' }}
                  >
                    {exp.period}
                  </div>
                  <div
                    className="text-[13px] leading-[1.7] mt-1"
                    style={{ color: 'var(--dim)' }}
                  >
                    {renderLineBreaks(exp.desc)}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </ScrollReveal>
      </div>
    </section>
  )
}
