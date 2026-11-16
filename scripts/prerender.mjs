// Bakes each route's HTML into the static build so text paints before any
// JavaScript runs; main.tsx then hydrates it. Runs after both vite builds.
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const dist = resolve(root, 'dist')
const ssrDir = resolve(root, 'dist-ssr')
const { render, getAllPosts } = await import(pathToFileURL(resolve(ssrDir, 'entry-server.js')).href)

const template = await readFile(resolve(dist, 'index.html'), 'utf8')
const ROOT_TAG = '<div id="root"></div>'
if (!template.includes(ROOT_TAG)) throw new Error(`prerender: ${ROOT_TAG} not found in dist/index.html`)

const escapeHtml = s => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')

// Unknown paths are rewritten here (vercel.json) and render client-side —
// serving them the homepage's markup would flash it before the 404.
await writeFile(resolve(dist, 'spa.html'), template)

const pages = [{ url: '/', file: 'index.html' }]
for (const post of getAllPosts()) {
  pages.push({
    url: `/blog/${post.slug}`,
    file: `blog/${post.slug}/index.html`,
    title: `${post.title} — Arpit Mishra`,
    description: post.description,
  })
}

for (const page of pages) {
  const body = await render(page.url)
  let html = template.replace(ROOT_TAG, `<div id="root">${body}</div>`)
  if (page.title) html = html.replace(/<title>[^<]*<\/title>/, `<title>${escapeHtml(page.title)}</title>`)
  if (page.description)
    html = html.replace(/(<meta name="description" content=")[^"]*(")/, `$1${escapeHtml(page.description)}$2`)
  const out = resolve(dist, page.file)
  await mkdir(dirname(out), { recursive: true })
  await writeFile(out, html)
  console.log(`prerendered ${page.url} → dist/${page.file} (${(body.length / 1024).toFixed(1)} KB)`)
}

await rm(ssrDir, { recursive: true, force: true })
