export interface BlogPost {
  id: string
  title: string
  description: string
  content: string
  image: string
  publishedAt: string
  updatedAt?: string
  tags: string[]
  readTime: number
  author: string
  slug: string
}

function parseFrontmatter(raw: string): { data: Record<string, unknown>; content: string } {
  const fmRegex = /^---\s*\n([\s\S]*?)\n---\s*\n?([\s\S]*)$/
  const match = raw.match(fmRegex)
  if (!match) return { data: {}, content: raw.trim() }
  const yaml = match[1]
  const content = match[2].trim()
  const data: Record<string, unknown> = {}
  // very small yaml parser for our frontmatter (handles strings, arrays)
  const lines = yaml.split('\n')
  for (const line of lines) {
    const idx = line.indexOf(':')
    if (idx === -1) continue
    const key = line.slice(0, idx).trim()
    let value: unknown = line.slice(idx + 1).trim()
    if (typeof value === 'string') {
      // strip quotes
      if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
        value = value.slice(1, -1)
      }
      // array like ["tag1", "tag2"] or [tag1, tag2]
      if (typeof value === 'string' && value.startsWith('[') && value.endsWith(']')) {
        const inner = value.slice(1, -1).trim()
        if (!inner) {
          value = []
        } else {
          value = inner.split(',').map(s => s.trim().replace(/^["']|["']$/g, '')).filter(Boolean)
        }
      }
    }
    data[key] = value
  }
  return { data, content }
}

function slugFromTitle(title: string, fallback: string): string {
  const slug =
    title
      ?.toLowerCase()
      .replace(/[^a-z0-9\s-]/g, '')
      .replace(/\s+/g, '-')
      .replace(/-+/g, '-')
      .trim() || fallback.replace(/\.md$/, '')
  return slug
}

// Vite's import.meta.glob for md files - bundle-time static include
// We handle both possible locations so `content/blog/*.md` at root and `src/content/blog/*.md` both work
const modulesRoot = import.meta.glob('/content/blog/*.md', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
const modulesRootAlt = import.meta.glob('../../content/blog/*.md', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
const modulesSrc = import.meta.glob('/src/content/blog/*.md', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
const modulesSrcAlt = import.meta.glob('../content/blog/*.md', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

const allModules: Record<string, string> = {
  ...modulesRoot,
  ...modulesRootAlt,
  ...modulesSrc,
  ...modulesSrcAlt,
}

function buildPosts(): BlogPost[] {
  const bySlug = new Map<string, BlogPost>()
  for (const [path, raw] of Object.entries(allModules)) {
    const { data, content } = parseFrontmatter(raw as string)
    const fileName = path.split('/').pop() || ''
    const title = (data.title as string) || ''
    const slug = slugFromTitle(title, fileName)
    // dedupe: same file can appear via multiple glob patterns (e.g. /content/blog/*.md and ../../content/blog/*.md)
    if (bySlug.has(slug)) continue
    const readTime = Math.max(1, Math.ceil(content.split(/\s+/).filter(Boolean).length / 200))
    bySlug.set(slug, {
      id: slug,
      title: title || slug,
      description: (data.description as string) || '',
      content,
      image: (data.image as string) || '',
      publishedAt: (data.publishedAt as string) || '',
      updatedAt: data.updatedAt as string | undefined,
      tags: (data.tags as string[]) || [],
      readTime,
      author: (data.author as string) || 'Arpit Mishra',
      slug,
    })
  }
  return Array.from(bySlug.values()).sort((a, b) => new Date(b.publishedAt).getTime() - new Date(a.publishedAt).getTime())
}

let cached: BlogPost[] | null = null
function getCached(): BlogPost[] {
  if (cached) return cached
  cached = buildPosts()
  return cached
}

export function getAllPosts(): BlogPost[] {
  return getCached()
}

export function getPostBySlug(slug: string): BlogPost | undefined {
  return getCached().find(p => p.slug === slug)
}

export function getAllSlugs(): string[] {
  return getCached().map(p => p.slug)
}

export function getRecentPosts(count = 3): BlogPost[] {
  return getCached().slice(0, count)
}
