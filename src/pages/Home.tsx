import Hero from '../components/Hero'
import Experience from '../components/Experience'
import Projects from '../components/Projects'
import Technologies from '../components/Technologies'
import Blog from '../components/Blog'
import Spotify from '../components/Spotify'
import Contact from '../components/Contact'

export default function Home() {
  return (
    <>
      <Hero />
      <Experience />
      <Projects />
      <Technologies />
      <Blog />
      <section className="py-20" id="contact">
        <div className="max-w-[1060px] mx-auto px-5 md:px-8">
          <Spotify />
          <Contact />
        </div>
      </section>
    </>
  )
}
