import { Link } from 'react-router-dom'
import { getAllPosts } from '../lib/blog'
import { formatDate } from '../lib/utils'

export default function Blog() {
  const posts = getAllPosts()

  return (
    <section className="py-20 border-b" style={{ borderColor: 'var(--border)' }} id="blog">
      <div className="max-w-[1060px] mx-auto px-5 md:px-8">
        <div className="mb-10">
          <h2 className="text-[clamp(24px,3vw,32px)] tracking-[3px] uppercase font-medium" style={{ color: 'var(--fg)' }}>
            Blog
          </h2>
          <span className="text-[12px] mt-1 inline-block" style={{ color: 'var(--accent)' }}>
            {posts.length} {posts.length === 1 ? 'post' : 'posts'}
          </span>
          <p className="text-[14px] leading-[1.8] max-w-[440px] mt-6" style={{ color: 'var(--dim)' }}>
            Notes on the code graph, benchmarks, and things I learn while building.
          </p>
        </div>

        {posts.length > 0 ? (
          <div className="grid" style={{ gap: '1px', background: 'var(--border)', border: '1px solid var(--border)' }}>
            {posts.map(post => (
              <Link
                key={post.slug}
                to={`/blog/${post.slug}`}
                className="group block p-6 md:p-7 transition-colors"
                style={{ background: 'var(--bg2)', textDecoration: 'none' }}
                onMouseEnter={e => (e.currentTarget.style.background = 'color-mix(in srgb, var(--bg2) 92%, var(--accent) 8%)')}
                onMouseLeave={e => (e.currentTarget.style.background = 'var(--bg2)')}
              >
                <div className="flex flex-col gap-3">
                  <div className="flex flex-wrap items-center gap-3">
                    <span className="text-[11px] tracking-[2px] uppercase" style={{ color: 'var(--dim)' }}>
                      {formatDate(post.publishedAt)}
                    </span>
                    <span className="text-[11px]" style={{ color: 'var(--border)' }}>
                      ·
                    </span>
                    <span className="text-[11px] tracking-[1.5px] uppercase" style={{ color: 'var(--accent)' }}>
                      {post.readTime} min
                    </span>
                  </div>

                  <h3
                    className="text-[18px] md:text-[20px] leading-[1.35] tracking-wide"
                    style={{ fontFamily: "'Geist Pixel', 'Geist Mono', monospace", color: 'var(--fg)' }}
                  >
                    {post.title}
                  </h3>

                  <p className="text-[13px] leading-[1.6] max-w-[640px]" style={{ color: 'var(--dim)' }}>
                    {post.description}
                  </p>
                </div>
              </Link>
            ))}
          </div>
        ) : (
          <div className="py-12 text-center" style={{ border: '1px solid var(--border)', background: 'var(--bg2)' }}>
            <p className="text-[14px]" style={{ color: 'var(--dim)' }}>
              No posts yet.
            </p>
          </div>
        )}
      </div>
    </section>
  )
}
