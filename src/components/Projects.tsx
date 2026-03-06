import { Fragment } from 'react'
import ScrollReveal from './ScrollReveal'

const projects = [
  {
    name: 'CareerWiki',
    desc: 'AI-powered platform for exploring career paths with interactive simulations and guidance.<br/>Integrated a personalized AI career consultant for tailored advice and analysis.<br/>Server-side logic with Next.js API routes and MongoDB for data storage.',
    tech: 'Next.js · Tailwind CSS · Google Gemini API · MongoDB',
  },
  {
    name: 'PropChain',
    desc: 'Real Estate marketplace on blockchain for validating real estate documents and buying and selling of land.<br/>Agentic AI based chat using LangGraph for interacting with the website.<br/>Implemented facial recognition based verification using Tesseract.',
    tech: 'Python · FastAPI · LangGraph · CopilotKit CoAgents · Solidity',
  },
  {
    name: 'RideMates',
    desc: 'Ride-sharing/carpool platform connecting drivers and riders.<br/>Built authentication, schema design, and ride-matching logic.<br/>Enhanced frontend UX with ratings, ride updates, and profile management.',
    tech: 'React · Node.js · MongoDB · Express · Firebase · Tailwind CSS',
  },
  {
    name: 'Speech to Insights',
    desc: 'Minimal frontend for converting voice input into text using OpenAI Whisper and generating insights using Hugging Face Inference Client pre voice integration in chatgpt era.',
    tech: 'FastAPI · React · Tailwind CSS · OpenAI Whisper',
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

export default function Projects() {
  return (
    <section className="py-20 border-b" style={{ borderColor: 'var(--border)' }} id="projects">
      <div className="max-w-[1060px] mx-auto px-5 md:px-8">
        <div className="mb-10">
          <h2
            className="text-[32px] tracking-[3px] uppercase font-light"
            style={{ color: 'var(--fg)' }}
          >
            Projects
          </h2>
          <span className="text-[12px] mt-1 inline-block" style={{ color: 'var(--accent)' }}>4 Projects</span>
        </div>

        <ScrollReveal stagger>
          <div
            className="grid grid-cols-1 md:grid-cols-2"
            style={{ gap: '1px', background: 'var(--border)', border: '1px solid var(--border)' }}
          >
            {projects.map((proj, i) => (
              <div
                key={i}
                className="reveal sweep-hover flex flex-col gap-5 p-6 md:p-8 relative"
                style={{ background: 'var(--bg2)' }}
                onMouseEnter={e =>
                  (e.currentTarget.style.background =
                    'color-mix(in srgb, var(--bg2) 92%, var(--accent) 8%)')
                }
                onMouseLeave={e => (e.currentTarget.style.background = 'var(--bg2)')}
              >
                <div className="flex items-center justify-between gap-2">
                  <div
                    className="text-[15px]"
                    style={{ color: 'var(--accent2)' }}
                  >
                    {proj.name}
                  </div>
                  <a
                    href="#"
                    className="text-[12px] px-2.5 py-1 tracking-wider transition-all hover:no-underline shrink-0"
                    style={{
                      color: 'var(--dim)',
                      border: '1px solid var(--border)',
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
                    source
                  </a>
                </div>
                <div className="flex flex-col gap-4">
                  <div
                    className="text-[13px] leading-[1.5]"
                    style={{ color: 'var(--dim)' }}
                  >
                    {renderLineBreaks(proj.desc)}
                  </div>
                  <div className="text-[12px] opacity-80" style={{ color: 'var(--dim)' }}>
                    {proj.tech}
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
