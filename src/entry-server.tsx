import { StrictMode } from 'react'
import { prerenderToNodeStream } from 'react-dom/static'
import { StaticRouter } from 'react-router-dom'
import App from './App'
import { getAllPosts } from './lib/blog'

export { getAllPosts }

/* Mirrors the tree in main.tsx (minus <Analytics />, which renders nothing)
   so hydration lines up. prerender waits for lazy routes, so blog posts come
   out fully rendered rather than as the Suspense fallback. */
export async function render(url: string): Promise<string> {
  const { prelude } = await prerenderToNodeStream(
    <StrictMode>
      <StaticRouter location={url}>
        <App />
      </StaticRouter>
    </StrictMode>,
  )
  // A Node Readable at runtime; the DOM lib types it as a web stream.
  const decoder = new TextDecoder()
  let html = ''
  for await (const chunk of prelude as unknown as AsyncIterable<Uint8Array | string>)
    html += typeof chunk === 'string' ? chunk : decoder.decode(chunk, { stream: true })
  return html + decoder.decode()
}
