import { useTheme } from './hooks/useTheme'
import Nav from './components/Nav'
import Hero from './components/Hero'
import Experience from './components/Experience'
import Projects from './components/Projects'
import Technologies from './components/Technologies'
import Spotify from './components/Spotify'
import Contact from './components/Contact'

export default function App() {
  const { theme, toggle } = useTheme()

  return (
    <>
      <div className="grid-bg" />
      <Nav theme={theme} onToggleTheme={toggle} />
      <Hero />
      <Experience />
      <Projects />
      <Technologies />
      <section className="py-20" id="contact">
        <div className="max-w-[1060px] mx-auto px-5 md:px-8">
          <Spotify />
          <Contact />
        </div>
      </section>
    </>
  )
}
