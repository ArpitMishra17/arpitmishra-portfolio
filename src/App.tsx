import { Suspense, lazy } from 'react'
import { Routes, Route, Link } from 'react-router-dom'
import { useTheme } from './hooks/useTheme'
import { useHashScroll } from './hooks/useHashScroll'
import Nav from './components/Nav'
import Home from './pages/Home'

// The blog route pulls in a 400-line markdown renderer; visitors who only ever
// see the homepage shouldn't pay for it.
const BlogPost = lazy(() => import('./pages/BlogPost'))

function NotFound() {
  return (
    <main className="max-w-[1060px] mx-auto px-5 md:px-8 pt-32 pb-16 min-h-[100svh]">
      <p className="text-[14px]" style={{ color: 'var(--dim)' }}>
        Page not found.
      </p>
      <Link to="/" className="inline-block mt-4 text-[13px] pressable hover-accent" style={{ color: 'var(--accent)' }}>
        ← home
      </Link>
    </main>
  )
}

export default function App() {
  const { theme, toggle } = useTheme()
  useHashScroll()

  return (
    <>
      <div className="grid-bg" />
      <Nav theme={theme} onToggleTheme={toggle} />
      <Suspense fallback={<div className="min-h-[100svh]" />}>
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/blog/:slug" element={<BlogPost />} />
          <Route path="*" element={<NotFound />} />
        </Routes>
      </Suspense>
    </>
  )
}
