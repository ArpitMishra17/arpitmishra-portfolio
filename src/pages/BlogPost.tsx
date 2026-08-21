import { useParams, Link } from 'react-router-dom'
import { useEffect } from 'react'
import { getPostBySlug, getAllPosts } from '../lib/blog'
import { formatDate, extractToc } from '../lib/utils'
import { BlogContent } from '../components/blog/BlogContent'
import { ReadingProgress } from '../components/blog/ReadingProgress'
import { TableOfContents } from '../components/blog/TableOfContents'

export default function BlogPost() {
  const { slug } = useParams<{ slug: string }>()
  const post = slug ? getPostBySlug(slug) : undefined

  useEffect(() => {
    window.scrollTo(0, 0)
  }, [slug])

  useEffect(() => {
    if (!post) return
    document.title = `${post.title} — Arpit Mishra`
    const meta = document.querySelector('meta[name="description"]')
    if (meta) meta.setAttribute('content', post.description)
    return () => {
      document.title = 'Arpit Mishra'
    }
  }, [post])

  if (!post) {
    return (
      <main className="max-w-[1060px] mx-auto px-5 md:px-8 pt-20 pb-16 min-h-screen">
        <p className="text-[14px]" style={{ color: 'var(--dim)' }}>
          Post not found.
        </p>
        <Link to="/#blog" className="text-[13px] mt-4 inline-block" style={{ color: 'var(--accent)' }}>
          ← all posts
        </Link>
      </main>
    )
  }

  const toc = extractToc(post.content)
  const allPosts = getAllPosts()
  const currentIdx = allPosts.findIndex(p => p.slug === slug)
  const prevPost = currentIdx < allPosts.length - 1 ? allPosts[currentIdx + 1] : null
  const nextPost = currentIdx > 0 ? allPosts[currentIdx - 1] : null
  const hasBanner = !!(post.image && !post.image.includes('default') && post.image.trim() !== '')

  return (
    <>
      <ReadingProgress />
      <main id="main" className="min-h-screen border-b" style={{ borderColor: 'var(--border)' }}>
        <div className="max-w-[1060px] mx-auto px-5 md:px-8 pt-16 sm:pt-20 pb-16">
          <article id="top">
            <div className="mb-8">
              <Link
                to="/#blog"
                className="text-[12px] tracking-wider inline-flex items-center gap-1.5"
                style={{ color: 'var(--dim)', textDecoration: 'none' }}
                onMouseEnter={e => (e.currentTarget.style.color = 'var(--accent)')}
                onMouseLeave={e => (e.currentTarget.style.color = 'var(--dim)')}
              >
                ← all posts
              </Link>
            </div>

            <header className="mb-10">
              <div className="flex flex-wrap items-center gap-3 mb-4">
                <span className="text-[11px] tracking-[2px] uppercase" style={{ color: 'var(--dim)' }}>
                  {formatDate(post.publishedAt)}
                </span>
                <span className="text-[11px]" style={{ color: 'var(--border)' }}>
                  ·
                </span>
                <span className="text-[11px] tracking-[1.5px] uppercase" style={{ color: 'var(--dim)' }}>
                  {post.readTime} min read
                </span>
              </div>

              <h1
                className="text-[clamp(28px,4vw,42px)] leading-[1.15] tracking-wide font-normal mb-4"
                style={{ fontFamily: "'Geist Pixel', 'Geist Mono', monospace", color: 'var(--fg)' }}
              >
                {post.title}
              </h1>

              <p className="text-[14px] leading-[1.8] max-w-[640px]" style={{ color: 'var(--dim)' }}>
                {post.description}
              </p>

              <div className="mt-6 flex items-center gap-2 text-[12px]" style={{ color: 'var(--dim)' }}>
                <span className="w-[5px] h-[5px] rounded-full shrink-0" style={{ background: 'var(--accent)' }} />
                <span>{post.author}</span>
              </div>
            </header>

            {hasBanner && (
              <figure className="mb-10 overflow-hidden" style={{ border: '1px solid var(--border)' }}>
                <div className="relative aspect-[16/9]" style={{ background: 'var(--bg2)' }}>
                  <img src={post.image} alt={post.title} loading="eager" className="w-full h-full object-cover block" />
                </div>
              </figure>
            )}

            {toc.length > 1 && (
              <details className="lg:hidden mb-8 group" style={{ border: '1px solid var(--border)', background: 'var(--bg2)' }}>
                <summary className="list-none cursor-pointer flex items-center justify-between px-6 py-4 text-[11px] tracking-[1.5px] uppercase" style={{ color: 'var(--dim)' }}>
                  <span>On this page</span>
                  <span className="text-[12px] transition-transform group-open:rotate-180" style={{ color: 'var(--dim)' }}>
                    ↓
                  </span>
                </summary>
                <div className="px-6 pb-5 pt-2" style={{ borderTop: '1px solid var(--border)' }}>
                  <TableOfContents items={toc} showHeader={false} />
                </div>
              </details>
            )}

            <div className={toc.length > 1 ? 'lg:grid lg:grid-cols-[minmax(0,680px)_220px] lg:gap-10' : ''}>
              <div className="min-w-0">
                <BlogContent content={post.content} />
                <div className="mt-12 h-px" style={{ background: 'var(--border)' }} />
              </div>

              {toc.length > 1 && (
                <aside className="hidden lg:block">
                  <div className="sticky top-20 p-5 max-h-[calc(100vh-5rem)] overflow-y-auto" style={{ border: '1px solid var(--border)', background: 'var(--bg2)' }}>
                    <TableOfContents items={toc} />
                  </div>
                </aside>
              )}
            </div>
          </article>

          {(prevPost || nextPost) && (
            <nav aria-label="Adjacent posts" className="mt-12">
              <div className="grid grid-cols-1 sm:grid-cols-2" style={{ gap: '1px', background: 'var(--border)', border: '1px solid var(--border)' }}>
                {prevPost ? (
                  <Link
                    to={`/blog/${prevPost.slug}`}
                    className="group block p-6 transition-colors"
                    style={{ background: 'var(--bg2)', textDecoration: 'none' }}
                    onMouseEnter={e => (e.currentTarget.style.background = 'color-mix(in srgb, var(--bg2) 92%, var(--accent) 8%)')}
                    onMouseLeave={e => (e.currentTarget.style.background = 'var(--bg2)')}
                  >
                    <span className="text-[11px] tracking-[1.5px] uppercase flex items-center gap-1.5 mb-2" style={{ color: 'var(--dim)' }}>
                      ← older
                    </span>
                    <span className="text-[14px] leading-[1.5]" style={{ color: 'var(--fg)' }}>
                      {prevPost.title}
                    </span>
                  </Link>
                ) : (
                  <div className="hidden sm:block" style={{ background: 'var(--bg2)' }} />
                )}
                {nextPost ? (
                  <Link
                    to={`/blog/${nextPost.slug}`}
                    className="group block p-6 sm:text-right transition-colors"
                    style={{ background: 'var(--bg2)', textDecoration: 'none' }}
                    onMouseEnter={e => (e.currentTarget.style.background = 'color-mix(in srgb, var(--bg2) 92%, var(--accent) 8%)')}
                    onMouseLeave={e => (e.currentTarget.style.background = 'var(--bg2)')}
                  >
                    <span className="text-[11px] tracking-[1.5px] uppercase flex items-center gap-1.5 sm:justify-end mb-2" style={{ color: 'var(--dim)' }}>
                      newer →
                    </span>
                    <span className="text-[14px] leading-[1.5]" style={{ color: 'var(--fg)' }}>
                      {nextPost.title}
                    </span>
                  </Link>
                ) : (
                  <div className="hidden sm:block" style={{ background: 'var(--bg2)' }} />
                )}
              </div>

              <div className="mt-6 flex items-center justify-between">
                <Link
                  to="/#blog"
                  className="text-[12px] tracking-wider"
                  style={{ color: 'var(--dim)', textDecoration: 'none' }}
                  onMouseEnter={e => (e.currentTarget.style.color = 'var(--accent)')}
                  onMouseLeave={e => (e.currentTarget.style.color = 'var(--dim)')}
                >
                  ← all posts
                </Link>
                <a
                  href="#top"
                  className="text-[12px] tracking-wider"
                  style={{ color: 'var(--dim)', textDecoration: 'none' }}
                  onMouseEnter={e => (e.currentTarget.style.color = 'var(--accent)')}
                  onMouseLeave={e => (e.currentTarget.style.color = 'var(--dim)')}
                >
                  back to top ↑
                </a>
              </div>
            </nav>
          )}
        </div>
      </main>
    </>
  )
}
