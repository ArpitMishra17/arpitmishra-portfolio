import { useEffect, useRef } from 'react'

export default function AsciiShader() {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const canvas = canvasRef.current!
    if (!canvas) return

    const ctx = canvas.getContext('2d')!
    if (!ctx) return

    const chars = ' .·:;=+*#%@'
    const fontSize = 14
    const lineHeight = 16
    const charWidth = 9
    let cols = 0
    let rows = 0
    let t = 0
    let animId: number

    // Permutation table for Perlin noise
    const perm = new Uint8Array(512)
    const p = new Uint8Array(256)
    for (let i = 0; i < 256; i++) p[i] = i
    for (let i = 255; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1))
      ;[p[i], p[j]] = [p[j], p[i]]
    }
    for (let i = 0; i < 512; i++) perm[i] = p[i & 255]

    function grad(hash: number, x: number, y: number) {
      const h = hash & 3
      return ((h & 1) ? -x : x) + ((h & 2) ? -y : y)
    }

    function fade(t: number) { return t * t * t * (t * (t * 6 - 15) + 10) }
    function lerp(a: number, b: number, t: number) { return a + t * (b - a) }

    function perlin(x: number, y: number) {
      const X = Math.floor(x) & 255, Y = Math.floor(y) & 255
      const xf = x - Math.floor(x), yf = y - Math.floor(y)
      const u = fade(xf), v = fade(yf)
      const aa = perm[perm[X] + Y], ab = perm[perm[X] + Y + 1]
      const ba = perm[perm[X + 1] + Y], bb = perm[perm[X + 1] + Y + 1]
      return lerp(
        lerp(grad(aa, xf, yf), grad(ba, xf - 1, yf), u),
        lerp(grad(ab, xf, yf - 1), grad(bb, xf - 1, yf - 1), u),
        v
      )
    }

    function fbm(x: number, y: number) {
      let val = 0, amp = 0.5
      for (let i = 0; i < 5; i++) {
        val += amp * perlin(x, y)
        x *= 2; y *= 2
        amp *= 0.5
      }
      return val * 0.5 + 0.5
    }

    function resize() {
      const parent = canvas.parentElement
      if (!parent) return
      const rect = parent.getBoundingClientRect()
      canvas.width = rect.width
      canvas.height = Math.max(rect.height, 480)
      cols = Math.floor(canvas.width / charWidth)
      rows = Math.floor(canvas.height / lineHeight)
    }

    function render() {
      const isDark = document.documentElement.getAttribute('data-theme') !== 'light'
      const bg = isDark ? '#08080a' : '#f2f0ec'
      ctx.fillStyle = bg
      ctx.fillRect(0, 0, canvas.width, canvas.height)
      ctx.font = `${fontSize}px "Geist Mono", monospace`
      ctx.textBaseline = 'top'

      const st = t * 0.02

      for (let row = 0; row < rows; row++) {
        for (let col = 0; col < cols; col++) {
          const nx = col / cols
          const ny = row / rows

          const n1 = fbm(nx * 3 + st, ny * 3 + st * 0.7)
          const n2 = fbm(nx * 2 - st * 0.5 + 10, ny * 2 + st * 0.3 + 10)
          const wave = Math.sin((nx + ny) * 8 - st * 2) * 0.1 + 0.1

          let intensity = n1 * 0.5 + n2 * 0.3 + wave

          const edgeFade =
            Math.min(1, nx * 2.5) *
            Math.min(1, (1 - nx) * 4) *
            Math.min(1, ny * 4) *
            Math.min(1, (1 - ny) * 4)
          intensity *= edgeFade
          intensity = Math.pow(Math.max(0, Math.min(1, intensity)), 0.7)

          if (intensity < 0.03) continue

          const charIdx = Math.floor(intensity * (chars.length - 1))
          const ch = chars[charIdx]

          if (isDark) {
            const r = Math.floor(100 + intensity * 80 * (1 - nx))
            const g = Math.floor(190 + intensity * 60 * (0.5 + nx * 0.5))
            const b = Math.floor(210 + intensity * 45)
            const alpha = 0.5 + intensity * 0.5
            ctx.fillStyle = `rgba(${r},${g},${b},${alpha})`
          } else {
            const lr = Math.floor(5 + intensity * 20)
            const lg = Math.floor(100 + intensity * 100)
            const lb = Math.floor(110 + intensity * 80)
            const alpha = 0.8 + intensity * 0.2
            ctx.fillStyle = `rgba(${lr},${lg},${lb},${alpha})`
          }

          ctx.fillText(ch, col * charWidth, row * lineHeight)
        }
      }

      t += 1
      animId = requestAnimationFrame(render)
    }

    resize()

    const resizeObserver = new ResizeObserver(resize)
    if (canvas.parentElement) {
      resizeObserver.observe(canvas.parentElement)
    }

    render()

    return () => {
      cancelAnimationFrame(animId)
      resizeObserver.disconnect()
    }
  }, [])

  return (
    <canvas
      ref={canvasRef}
      className="block rounded-full"
      style={{ width: '80%', aspectRatio: '1' }}
    />
  )
}
