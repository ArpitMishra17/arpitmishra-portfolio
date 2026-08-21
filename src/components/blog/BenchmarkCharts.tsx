import { useState } from 'react'

type Repo = 'babylon' | 'vscode'
type MetricKey = 'symFound' | 'symCorrect' | 'connFound' | 'connCorrect' | 'callPrec' | 'callRec' | 'target'

interface ToolData {
  name: string
  symFound: number
  symCorrect: number
  connFound: number
  connCorrect: number
  callPrec: number | null
  callRec: number | null
  target: number | null
}

const DATA: Record<Repo, { label: string; tools: ToolData[]; caps: Record<string, string> }> = {
  babylon: {
    label: 'BabylonJS',
    tools: [
      { name: 'Vyazen', symFound: 99.4, symCorrect: 98.2, connFound: 79.7, connCorrect: 90.4, callPrec: 85.1, callRec: 74.5, target: 99.9 },
      { name: 'GitNexus', symFound: 83.5, symCorrect: 77.8, connFound: 39.9, connCorrect: 49.3, callPrec: 48.2, callRec: 62.2, target: 97.2 },
      { name: 'Graphify', symFound: 52.7, symCorrect: 69.4, connFound: 58.5, connCorrect: 78.8, callPrec: 67.8, callRec: 33.7, target: 98.4 },
      { name: 'Potpie', symFound: 48.1, symCorrect: 70.2, connFound: 0, connCorrect: 0, callPrec: null, callRec: 0, target: null },
    ],
    caps: {
      symbols: 'Symbols emitted — Vyazen 79,324 · GitNexus 80,773 · Graphify 56,412 · Potpie 54,034. Scored against ~80,000 real symbols.',
      connections: 'Relationships emitted — Vyazen 152,639 · GitNexus 259,345 · Graphify 133,845 · Potpie 134,740. Potpie emits no CALLS, IMPORTS, EXTENDS or IMPLEMENTS edges for TypeScript.',
      calls: '138,575 call sites seen by the compiler; 83.6% directly resolved, 15.8% excluded as unresolvable. Potpie emits no call edges, so precision and target accuracy do not apply.',
    },
  },
  vscode: {
    label: 'VS Code',
    tools: [
      { name: 'Vyazen', symFound: 99.2, symCorrect: 90.0, connFound: 84.3, connCorrect: 89.1, callPrec: 92.2, callRec: 79.1, target: 99.9 },
      { name: 'GitNexus', symFound: 69.4, symCorrect: 77.1, connFound: 31.4, connCorrect: 47.5, callPrec: 45.7, callRec: 46.4, target: 94.8 },
      { name: 'Graphify', symFound: 57.5, symCorrect: 85.1, connFound: 57.6, connCorrect: 79.9, callPrec: 84.7, callRec: 36.4, target: 97.2 },
      { name: 'Potpie', symFound: 53.7, symCorrect: 90.9, connFound: 0, connCorrect: 0, callPrec: null, callRec: 0, target: null },
    ],
    caps: {
      symbols: 'Symbols emitted — Vyazen 155,531 · GitNexus 132,925 · Graphify 94,260 · Potpie 85,658. Across 5,108 TS/JS files in 84 projects.',
      connections: 'Relationships emitted — Vyazen 400,063 · GitNexus 522,198 · Graphify 338,945 · Potpie 320,001. Potpie emits no CALLS, IMPORTS, EXTENDS or IMPLEMENTS edges for TypeScript.',
      calls: '393,675 call sites seen by the compiler; 87.0% directly resolved, 11.9% excluded as unresolvable. Potpie emits no call edges, so precision and target accuracy do not apply.',
    },
  },
}

const LOGOS: Record<string, string> = {
  Vyazen: '/images/logos/vyazen.webp',
  GitNexus: '/images/logos/akon.webp',
  Graphify: '/images/logos/graphify.webp',
  Potpie: '/images/logos/potpie.webp',
}

function ChartRow({ tool, value, repoLabel }: { tool: ToolData; value: number | null; repoLabel: string }) {
  const isNA = value === null
  const width = isNA ? 0 : value
  const logo = LOGOS[tool.name]
  return (
    <div className="grid items-center gap-4 py-2.5" style={{ gridTemplateColumns: '110px minmax(0,1fr) 52px' }}>
      <span className="flex items-center gap-2 justify-start min-w-0" title={tool.name}>
        {logo && (
          <img
            src={logo}
            alt=""
            aria-hidden="true"
            className="h-[16px] w-[16px] object-contain shrink-0"
            loading="lazy"
          />
        )}
        <span className="text-[13px] truncate" style={{ color: 'var(--fg)' }}>
          {tool.name}
        </span>
      </span>
      <div className="h-[4px] overflow-hidden rounded-full" style={{ background: 'var(--border)' }}>
        <div
          className="h-full transition-[width] duration-500"
          style={{
            width: `${width}%`,
            background: 'var(--dim)',
            opacity: 0.9,
          }}
        />
      </div>
      <span
        className="text-right font-mono text-[12px] tabular-nums"
        style={{ color: 'var(--dim)', opacity: isNA ? 0.6 : 1 }}
        title={isNA ? `${tool.name} — emits no call edges for TypeScript` : `${tool.name} — ${value?.toFixed(1)} on ${repoLabel}`}
      >
        {isNA ? 'n/a' : value!.toFixed(1)}
      </span>
    </div>
  )
}

function Chart({
  title,
  panels,
  cap,
  repo,
  setRepo,
  data,
}: {
  title: string
  panels: { label: string; metric: MetricKey; desc: string }[]
  cap: string
  repo: Repo
  setRepo: (r: Repo) => void
  data: (typeof DATA)[Repo]
}) {
  return (
    <div className="my-8 overflow-hidden" style={{ border: '1px solid var(--border)', background: 'var(--bg2)' }}>
      <div className="p-6">
        <div className="flex flex-wrap gap-3 items-center justify-between mb-6">
          <span className="text-[11px] tracking-[1.5px] uppercase" style={{ color: 'var(--dim)' }}>
            {title}
          </span>
          <div className="inline-flex" style={{ border: '1px solid var(--border)', background: 'var(--bg)' }}>
            {(['babylon', 'vscode'] as Repo[]).map(r => (
              <button
                key={r}
                type="button"
                onClick={() => setRepo(r)}
                aria-pressed={repo === r}
                className="px-3 py-1.5 text-[11px] tracking-[1.5px] uppercase transition-colors"
                style={{
                  background: repo === r ? 'var(--accent)' : 'transparent',
                  color: repo === r ? 'var(--bg)' : 'var(--dim)',
                  border: 'none',
                  cursor: 'pointer',
                }}
              >
                {r === 'babylon' ? 'BabylonJS' : 'VS Code'}
              </button>
            ))}
          </div>
        </div>

        {panels.map(panel => (
          <div key={panel.metric} className="mb-6 last:mb-0">
            <div className="mb-3">
              <span className="text-[13px] font-medium" style={{ color: 'var(--fg)' }}>
                {panel.label}
              </span>
              <span className="text-[12px] ml-2" style={{ color: 'var(--dim)' }}>
                {panel.desc}
              </span>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '1px', background: 'var(--border)', border: '1px solid var(--border)' }}>
              {data.tools.map(tool => (
                <div key={tool.name} style={{ background: 'var(--bg2)' }} className="px-4">
                  <ChartRow tool={tool} value={tool[panel.metric] as number | null} repoLabel={data.label} />
                </div>
              ))}
            </div>
          </div>
        ))}

        <div className="mt-6 pt-4 text-[12px] leading-[1.6]" style={{ borderTop: '1px solid var(--border)', color: 'var(--dim)' }}>
          {cap}
        </div>
      </div>
    </div>
  )
}

export function BenchmarkCharts() {
  const [repo, setRepo] = useState<Repo>('babylon')
  const d = DATA[repo]
  return (
    <div className="my-8">
      <Chart
        title="Symbols"
        repo={repo}
        setRepo={setRepo}
        data={d}
        cap={d.caps.symbols}
        panels={[
          { label: 'Symbols found', metric: 'symFound', desc: 'of the real symbols, how many were found' },
          { label: 'Symbols correct', metric: 'symCorrect', desc: 'of the claimed symbols, how many are real' },
        ]}
      />
      <Chart
        title="Connections"
        repo={repo}
        setRepo={setRepo}
        data={d}
        cap={d.caps.connections}
        panels={[
          { label: 'Connections found', metric: 'connFound', desc: 'of the real relationships, how many were found' },
          { label: 'Connections correct', metric: 'connCorrect', desc: 'of the claimed relationships, how many are real' },
        ]}
      />
      <Chart
        title="Calls"
        repo={repo}
        setRepo={setRepo}
        data={d}
        cap={d.caps.calls}
        panels={[
          { label: 'Call precision', metric: 'callPrec', desc: 'of the claimed calls, how many are real' },
          { label: 'Call recall', metric: 'callRec', desc: 'of the real calls, how many were found' },
          { label: 'Target accuracy', metric: 'target', desc: 'of the correct edges, how many hit the exact file and line' },
        ]}
      />
    </div>
  )
}

export function ChartSymbols({ repo, setRepo }: { repo: Repo; setRepo: (r: Repo) => void }) {
  const d = DATA[repo]
  return (
    <Chart
      title="Symbols"
      repo={repo}
      setRepo={setRepo}
      data={d}
      cap={d.caps.symbols}
      panels={[
        { label: 'Symbols found', metric: 'symFound', desc: 'of the real symbols, how many were found' },
        { label: 'Symbols correct', metric: 'symCorrect', desc: 'of the claimed symbols, how many are real' },
      ]}
    />
  )
}
export function ChartConnections({ repo, setRepo }: { repo: Repo; setRepo: (r: Repo) => void }) {
  const d = DATA[repo]
  return (
    <Chart
      title="Connections"
      repo={repo}
      setRepo={setRepo}
      data={d}
      cap={d.caps.connections}
      panels={[
        { label: 'Connections found', metric: 'connFound', desc: 'of the real relationships, how many were found' },
        { label: 'Connections correct', metric: 'connCorrect', desc: 'of the claimed relationships, how many are real' },
      ]}
    />
  )
}
export function ChartCalls({ repo, setRepo }: { repo: Repo; setRepo: (r: Repo) => void }) {
  const d = DATA[repo]
  return (
    <Chart
      title="Calls"
      repo={repo}
      setRepo={setRepo}
      data={d}
      cap={d.caps.calls}
      panels={[
        { label: 'Call precision', metric: 'callPrec', desc: 'of the claimed calls, how many are real' },
        { label: 'Call recall', metric: 'callRec', desc: 'of the real calls, how many were found' },
        { label: 'Target accuracy', metric: 'target', desc: 'of the correct edges, how many hit the exact file and line' },
      ]}
    />
  )
}
