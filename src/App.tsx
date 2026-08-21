import { Routes, Route } from 'react-router-dom'
import { useTheme } from './hooks/useTheme'
import Nav from './components/Nav'
import Home from './pages/Home'
import BlogPost from './pages/BlogPost'

export default function App() {
  const { theme, toggle } = useTheme()

  return (
    <>
      <div className="grid-bg" />
      <Nav theme={theme} onToggleTheme={toggle} />
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/blog/:slug" element={<BlogPost />} />
      </Routes>
    </>
  )
}
