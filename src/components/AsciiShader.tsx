import { useEffect, useRef } from 'react'

export default function AsciiShader() {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    let mounted = true

    // Bayer 8x8 ordered dithering matrix (normalized 0–1)
    const bayer8 = new Float32Array(64)
    {
      const b4 = [
         0, 32,  8, 40,  2, 34, 10, 42,
        48, 16, 56, 24, 50, 18, 58, 26,
        12, 44,  4, 36, 14, 46,  6, 38,
        60, 28, 52, 20, 62, 30, 54, 22,
         3, 35, 11, 43,  1, 33,  9, 41,
        51, 19, 59, 27, 49, 17, 57, 25,
        15, 47,  7, 39, 13, 45,  5, 37,
        63, 31, 55, 23, 61, 29, 53, 21,
      ]
      for (let i = 0; i < 64; i++) bayer8[i] = b4[i] / 64
    }

    const img = new Image()
    img.src = '/hero.jpg'
    img.onload = () => {
      if (!mounted) return

      const PIX = 3 // dither cell size — each NxN block gets one dithered color
      const levels = 5 // color quantization levels per channel

      function resize() {
        const el = canvas.parentElement
        if (!el) return
        const rect = el.getBoundingClientRect()
        canvas.width = Math.floor(rect.width)
        canvas.height = Math.max(Math.floor(rect.height), 480)
      }

      function render() {
        if (!mounted) return

        const W = canvas.width
        const H = canvas.height

        // Draw the source image scaled to fill canvas (cover)
        const imgAspect = img.width / img.height
        const canAspect = W / H
        let sw: number, sh: number, sx: number, sy: number
        if (imgAspect > canAspect) {
          sh = img.height
          sw = img.height * canAspect
          sx = (img.width - sw) / 2
          sy = 0
        } else {
          sw = img.width
          sh = img.width / canAspect
          sx = 0
          sy = (img.height - sh) / 2
        }
        ctx.drawImage(img, sx, sy, sw, sh, 0, 0, W, H)

        // Read pixels
        const imageData = ctx.getImageData(0, 0, W, H)
        const d = imageData.data

        // Apply ordered dithering at PIX resolution
        const gw = Math.ceil(W / PIX)
        const gh = Math.ceil(H / PIX)

        for (let gy = 0; gy < gh; gy++) {
          for (let gx = 0; gx < gw; gx++) {
            // Sample center pixel of the cell
            const cx = Math.min(gx * PIX + (PIX >> 1), W - 1)
            const cy = Math.min(gy * PIX + (PIX >> 1), H - 1)
            const si = (cy * W + cx) * 4
            const r = d[si] / 255
            const g = d[si + 1] / 255
            const b = d[si + 2] / 255

            // Bayer threshold for this cell
            const threshold = bayer8[(gy & 7) * 8 + (gx & 7)] - 0.5

            // Dither each channel: add threshold, then quantize to `levels`
            const spread = 1 / levels
            const dr = Math.max(0, Math.min(1, r + threshold * spread))
            const dg = Math.max(0, Math.min(1, g + threshold * spread))
            const db = Math.max(0, Math.min(1, b + threshold * spread))

            // Quantize
            const qr = Math.round(dr * (levels - 1)) / (levels - 1)
            const qg = Math.round(dg * (levels - 1)) / (levels - 1)
            const qb = Math.round(db * (levels - 1)) / (levels - 1)

            const fr = Math.floor(qr * 255)
            const fg = Math.floor(qg * 255)
            const fb = Math.floor(qb * 255)

            // Fill entire PIX x PIX cell with dithered color
            for (let dy = 0; dy < PIX; dy++) {
              const fy = gy * PIX + dy
              if (fy >= H) continue
              for (let dx = 0; dx < PIX; dx++) {
                const fx = gx * PIX + dx
                if (fx >= W) continue
                const idx = (fy * W + fx) * 4
                d[idx] = fr
                d[idx + 1] = fg
                d[idx + 2] = fb
              }
            }
          }
        }

        ctx.putImageData(imageData, 0, 0)
      }

      resize()
      render()

      const ro = new ResizeObserver(() => { resize(); render() })
      if (canvas.parentElement) ro.observe(canvas.parentElement)

      return () => { ro.disconnect() }
    }

    return () => { mounted = false }
  }, [])

  return (
    <canvas
      ref={canvasRef}
      className="block"
      style={{ width: '100%', height: '100%', objectFit: 'cover' }}
    />
  )
}
