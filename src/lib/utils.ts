export function formatDate(dateString: string): string {
  if (!dateString) return ''
  const date = new Date(dateString)
  if (isNaN(date.getTime())) return dateString
  return `${date.getFullYear()}.${String(date.getMonth() + 1).padStart(2, '0')}.${String(date.getDate()).padStart(2, '0')}`
}

export function slugify(s: string): string {
  return s
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
}

export interface TocItem {
  level: 2 | 3 | 4
  text: string
  slug: string
}

export function extractToc(markdown: string): TocItem[] {
  const items: TocItem[] = []
  const seen = new Map<string, number>()
  const lines = markdown.split('\n')
  let inFence = false
  for (const line of lines) {
    if (line.startsWith('```')) {
      inFence = !inFence
      continue
    }
    if (inFence) continue
    let match: RegExpMatchArray | null
    let level: 2 | 3 | 4 | null = null
    let text = ''
    if ((match = line.match(/^##\s+(.+?)\s*$/))) {
      level = 2
      text = match[1]
    } else if ((match = line.match(/^###\s+(.+?)\s*$/))) {
      level = 3
      text = match[1]
    } else if ((match = line.match(/^####\s+(.+?)\s*$/))) {
      level = 4
      text = match[1]
    }
    if (level && text) {
      const base = slugify(text) || `section-${items.length + 1}`
      const count = seen.get(base) ?? 0
      const slug = count === 0 ? base : `${base}-${count}`
      seen.set(base, count + 1)
      items.push({ level, text, slug })
    }
  }
  return items
}
