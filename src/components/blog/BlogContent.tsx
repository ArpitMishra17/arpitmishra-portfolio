import { useState } from 'react'
import { slugify } from '../../lib/utils'
import { BenchmarkCharts, ChartSymbols, ChartConnections, ChartCalls } from './BenchmarkCharts'

const TOOL_LOGOS: Record<string, string> = {
  Vyazen: '/images/logos/vyazen.webp',
  GitNexus: '/images/logos/akon.webp',
  Graphify: '/images/logos/graphify.webp',
  Potpie: '/images/logos/potpie.webp',
}

interface BlogContentProps {
  content: string
}

type Repo = 'babylon' | 'vscode'

export function BlogContent({ content }: BlogContentProps) {
  const [repo, setRepo] = useState<Repo>('babylon')

  const renderInlineMarkdown = (text: string): React.ReactNode[] => {
    const elements: React.ReactNode[] = []
    let keyIndex = 0
    const inlineRegex =
      /\[([^\]]+)\]\(([^)]+)\)|\*\*(.+?)\*\*|__(.+?)__|(?<!\*)\*(?!\*)(.+?)(?<!\*)\*(?!\*)|(?<![A-Za-z0-9_])_(?!_)(.+?)(?<!_)_(?![A-Za-z0-9_])|`([^`]+)`/g
    let match: RegExpExecArray | null
    let lastIndex = 0

    while ((match = inlineRegex.exec(text)) !== null) {
      if (match.index > lastIndex) {
        elements.push(<span key={`t-${keyIndex++}`}>{text.slice(lastIndex, match.index)}</span>)
      }
      if (match[1] !== undefined && match[2] !== undefined) {
        const isExternal = /^https?:\/\//i.test(match[2])
        elements.push(
          <a
            key={`a-${keyIndex++}`}
            href={match[2]}
            {...(isExternal ? { target: '_blank', rel: 'noopener noreferrer' } : {})}
            style={{ color: 'var(--accent)' }}
          >
            {match[1]}
          </a>,
        )
      } else if (match[3] !== undefined || match[4] !== undefined) {
        elements.push(
          <strong key={`b-${keyIndex++}`} style={{ color: 'var(--fg)', fontWeight: 500 }}>
            {match[3] ?? match[4]}
          </strong>,
        )
      } else if (match[5] !== undefined || match[6] !== undefined) {
        elements.push(
          <em key={`i-${keyIndex++}`} style={{ color: 'var(--dim)', fontStyle: 'italic' }}>
            {match[5] ?? match[6]}
          </em>,
        )
      } else if (match[7] !== undefined) {
        elements.push(
          <code
            key={`c-${keyIndex++}`}
            className="px-1.5 py-0.5 text-[12px] font-mono"
            style={{ background: 'var(--bg2)', border: '1px solid var(--border)', color: 'var(--accent2)' }}
          >
            {match[7]}
          </code>,
        )
      }
      lastIndex = match.index + match[0].length
    }
    if (lastIndex < text.length) {
      elements.push(<span key={`t-${keyIndex++}`}>{text.slice(lastIndex)}</span>)
    }
    return elements.length > 0 ? elements : [<span key="text">{text}</span>]
  }

  const renderMarkdown = (md: string) => {
    const lines = md.split('\n')
    const elements: React.ReactNode[] = []
    let inCodeBlock = false
    let codeBlockContent: string[] = []
    let codeBlockLang = ''
    let currentList: { type: 'ul' | 'ol'; items: React.ReactNode[] } | null = null
    let paragraphIndex = 0
    const slugCounts = new Map<string, number>()

    const uniqueSlug = (text: string) => {
      const base = slugify(text) || `section-${elements.length + 1}`
      const count = slugCounts.get(base) ?? 0
      slugCounts.set(base, count + 1)
      return count === 0 ? base : `${base}-${count}`
    }

    const flushList = () => {
      if (!currentList) return
      const { type, items } = currentList
      if (type === 'ul') {
        elements.push(
          <ul key={`list-${elements.length}`} className="my-6 flex flex-col gap-3 list-none m-0 p-0">
            {items}
          </ul>,
        )
      } else {
        elements.push(
          <ol key={`list-${elements.length}`} className="my-6 flex flex-col gap-3 list-none m-0 p-0">
            {items}
          </ol>,
        )
      }
      currentList = null
    }

    const parseTableRow = (row: string): string[] => {
      let s = row.trim()
      if (s.startsWith('|')) s = s.slice(1)
      if (s.endsWith('|')) s = s.slice(0, -1)
      return s.split('|').map(c => c.trim())
    }

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i]
      const trimmed = line.trim()

      if (trimmed === ':::benchmark-charts' || trimmed === ':::charts' || trimmed === '[[CHARTS]]' || trimmed === '::charts::') {
        flushList()
        elements.push(<BenchmarkCharts key={`charts-${i}`} />)
        continue
      }
      if (trimmed === ':::chart-symbols') {
        flushList()
        elements.push(<ChartSymbols key={`cs-${i}`} repo={repo} setRepo={setRepo} />)
        continue
      }
      if (trimmed === ':::chart-connections') {
        flushList()
        elements.push(<ChartConnections key={`cc-${i}`} repo={repo} setRepo={setRepo} />)
        continue
      }
      if (trimmed === ':::chart-calls') {
        flushList()
        elements.push(<ChartCalls key={`cl-${i}`} repo={repo} setRepo={setRepo} />)
        continue
      }

      if (line.startsWith('```')) {
        if (inCodeBlock) {
          flushList()
          elements.push(
            <div key={`code-${i}`} className="my-8 overflow-hidden" style={{ border: '1px solid var(--border)', background: 'var(--bg2)' }}>
              {codeBlockLang && (
                <div className="px-4 py-2 text-[11px] tracking-[1.5px] uppercase" style={{ color: 'var(--dim)', borderBottom: '1px solid var(--border)' }}>
                  {codeBlockLang}
                </div>
              )}
              <pre className="p-4 overflow-x-auto text-[13px] leading-relaxed" style={{ background: 'var(--bg2)' }}>
                <code className="font-mono whitespace-pre" style={{ color: 'var(--fg)' }}>
                  {codeBlockContent.join('\n')}
                </code>
              </pre>
            </div>,
          )
          codeBlockContent = []
          codeBlockLang = ''
          inCodeBlock = false
        } else {
          flushList()
          codeBlockLang = line.slice(3).trim()
          inCodeBlock = true
        }
        continue
      }

      if (inCodeBlock) {
        codeBlockContent.push(line)
        continue
      }

      if (line.includes('|') && i + 1 < lines.length && /^\s*\|?[-:\s|]+\|?\s*$/.test(lines[i + 1]) && lines[i + 1].includes('-')) {
        flushList()
        const headers = parseTableRow(line)
        i += 1
        const rows: string[][] = []
        let j = i + 1
        while (j < lines.length && lines[j].trim().includes('|') && lines[j].trim() !== '' && !lines[j].startsWith('#') && !lines[j].startsWith('```')) {
          const t = lines[j].trim()
          if (!t.includes('|')) break
          if (/^\s*\|?[-:\s|]+\|?\s*$/.test(t) && t.replace(/[-:\s|]/g, '').length === 0) break
          rows.push(parseTableRow(lines[j]))
          j++
        }
        const isScorecard = headers.length >= 8
        const hasLogoColumn = rows.some(row => TOOL_LOGOS[row[0]] !== undefined)
        elements.push(
          <div key={`tbl-${i}`} className="my-8 overflow-x-auto -mx-5 md:mx-0" style={{ border: '1px solid var(--border)', background: 'var(--border)' }}>
            <table className="w-full border-collapse text-[13px]" style={{ minWidth: isScorecard ? '760px' : '100%', tableLayout: isScorecard ? 'fixed' : 'auto', background: 'var(--bg2)' }}>
              <thead>
                <tr>
                  {headers.map((h, idx) => (
                    <th
                      key={idx}
                      className={`text-left px-4 py-3 text-[11px] tracking-[1.5px] uppercase whitespace-nowrap ${isScorecard && idx > 0 ? 'w-[72px]' : idx === 0 && hasLogoColumn ? 'w-[150px]' : ''}`}
                      style={{ color: 'var(--dim)', borderBottom: '1px solid var(--border)', background: 'var(--bg2)' }}
                    >
                      {renderInlineMarkdown(h)}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {rows.map((row, rIdx) => (
                  <tr key={rIdx} style={{ background: 'var(--bg2)' }}>
                    {row.map((cell, cIdx) => {
                      const logo = cIdx === 0 && hasLogoColumn ? TOOL_LOGOS[cell] : undefined
                      return (
                        <td
                          key={cIdx}
                          className={`px-4 py-3 align-middle ${cIdx === 0 ? 'whitespace-nowrap font-medium' : ''} ${isScorecard && cIdx > 1 ? 'text-right font-mono text-[12px] tabular-nums overflow-hidden text-ellipsis' : ''}`}
                          style={{
                            color: cIdx === 0 ? 'var(--fg)' : 'var(--dim)',
                            borderBottom: rIdx < rows.length - 1 ? '1px solid var(--border)' : 'none',
                          }}
                        >
                          {logo ? (
                            <span className="inline-flex items-center gap-2.5 max-w-full">
                              <img src={logo} alt="" aria-hidden="true" className="h-[18px] w-[18px] object-contain shrink-0" loading="lazy" />
                              <span className="truncate">{cell}</span>
                            </span>
                          ) : (
                            renderInlineMarkdown(cell)
                          )}
                        </td>
                      )
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>,
        )
        i = j - 1
        continue
      }

      if (line.startsWith('#### ')) {
        flushList()
        const text = line.substring(5)
        const id = uniqueSlug(text)
        elements.push(
          <h4 key={i} id={id} className="text-[12px] tracking-[1.5px] uppercase font-medium mt-8 mb-3" style={{ color: 'var(--dim)' }}>
            {text}
          </h4>,
        )
        continue
      }
      if (line.startsWith('### ')) {
        flushList()
        const text = line.substring(4)
        const id = uniqueSlug(text)
        elements.push(
          <h3 key={i} id={id} className="text-[13px] tracking-[1.5px] uppercase font-medium mt-10 mb-4" style={{ color: 'var(--fg)' }}>
            {text}
          </h3>,
        )
        continue
      }
      if (line.startsWith('## ')) {
        flushList()
        const text = line.substring(3)
        const id = uniqueSlug(text)
        elements.push(
          <h2
            key={i}
            id={id}
            className="text-[20px] md:text-[22px] leading-[1.3] tracking-wide mt-12 mb-4"
            style={{ fontFamily: "'Geist Pixel', 'Geist Mono', monospace", color: 'var(--fg)' }}
          >
            {text}
          </h2>,
        )
        continue
      }
      if (line.startsWith('# ')) {
        flushList()
        const text = line.substring(2)
        const id = uniqueSlug(text)
        elements.push(
          <h1
            key={i}
            id={id}
            className="text-[24px] md:text-[28px] leading-tight tracking-wide mt-12 mb-6"
            style={{ fontFamily: "'Geist Pixel', 'Geist Mono', monospace", color: 'var(--fg)' }}
          >
            {text}
          </h1>,
        )
        continue
      }

      const imageMatch = line.match(/^!\[([^\]]*)\]\(([^)]+)\)$/)
      if (imageMatch) {
        flushList()
        elements.push(
          <figure key={i} className="my-8">
            <div className="overflow-hidden" style={{ border: '1px solid var(--border)', background: 'var(--bg2)' }}>
              <img src={imageMatch[2]} alt={imageMatch[1]} loading="lazy" decoding="async" className="w-full h-auto block" />
            </div>
            {imageMatch[1] && (
              <figcaption className="text-[11px] tracking-[1.5px] uppercase mt-3 flex items-center gap-2" style={{ color: 'var(--dim)' }}>
                <span style={{ color: 'var(--border)' }}>→</span>
                <span>{imageMatch[1]}</span>
              </figcaption>
            )}
          </figure>,
        )
        continue
      }

      if (line.startsWith('> ')) {
        flushList()
        elements.push(
          <blockquote
            key={i}
            className="my-8 px-5 py-4 text-[14px] leading-[1.7]"
            style={{ borderLeft: '2px solid var(--accent)', background: 'var(--bg2)', color: 'var(--dim)' }}
          >
            {renderInlineMarkdown(line.substring(2))}
          </blockquote>,
        )
        continue
      }

      if (trimmed === '---' || trimmed === '***') {
        flushList()
        elements.push(<div key={i} className="my-10 h-px" style={{ background: 'var(--border)' }} aria-hidden="true" />)
        continue
      }

      if (trimmed === '') {
        flushList()
        continue
      }

      if (trimmed.startsWith('<details') || trimmed.startsWith('<summary') || trimmed.startsWith('</details') || trimmed.startsWith('</summary')) {
        flushList()
        continue
      }
      if (trimmed.startsWith('<div') && trimmed.includes('scroll')) {
        continue
      }

      if (line.match(/^[-*] /)) {
        if (!currentList || currentList.type !== 'ul') {
          flushList()
          currentList = { type: 'ul', items: [] }
        }
        currentList.items.push(
          <li key={i} className="flex items-start gap-2 text-[14px] leading-[1.7]" style={{ color: 'var(--dim)' }}>
            <span className="mt-[8px] shrink-0 w-[5px] h-[5px] rounded-full" style={{ background: 'var(--accent)' }} />
            <span>{renderInlineMarkdown(line.replace(/^[-*] /, ''))}</span>
          </li>,
        )
        continue
      }

      const olMatch = line.match(/^(\d+)\. /)
      if (olMatch) {
        if (!currentList || currentList.type !== 'ol') {
          flushList()
          currentList = { type: 'ol', items: [] }
        }
        currentList.items.push(
          <li key={i} className="flex items-start gap-2 text-[14px] leading-[1.7]" style={{ color: 'var(--dim)' }}>
            <span className="shrink-0 text-[12px] tabular-nums mt-0.5" style={{ color: 'var(--accent)' }}>
              {olMatch[1]}.
            </span>
            <span>{renderInlineMarkdown(line.replace(/^\d+\. /, ''))}</span>
          </li>,
        )
        continue
      }

      flushList()
      const isLede = paragraphIndex === 0
      elements.push(
        <p key={i} className={isLede ? 'text-[15px] leading-[1.7] mb-6' : 'text-[14px] leading-[1.8] mb-6'} style={{ color: isLede ? 'var(--fg)' : 'var(--dim)' }}>
          {renderInlineMarkdown(line)}
        </p>,
      )
      paragraphIndex++
    }

    flushList()
    return elements
  }

  return <div>{renderMarkdown(content)}</div>
}
