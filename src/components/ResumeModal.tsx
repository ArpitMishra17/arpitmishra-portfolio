import { useCallback, useEffect, useRef, useState } from 'react'
import type { KeyboardEvent as ReactKeyboardEvent } from 'react'
import { createPortal } from 'react-dom'
import {
  AmbientLight,
  BoxGeometry,
  CanvasTexture,
  DirectionalLight,
  MathUtils,
  Mesh,
  MeshBasicMaterial,
  MeshLambertMaterial,
  PerspectiveCamera,
  PlaneGeometry,
  SRGBColorSpace,
  Scene,
  TextureLoader,
  WebGLRenderer,
} from 'three'
import type { Texture } from 'three'
import { RESUME_IMG, RESUME_PDF } from '../lib/resume'

/* US Letter at 96dpi — what "100%" means in a PDF viewer. */
const PAGE_W = 816
const PAGE_H = 1056
const ASPECT = PAGE_W / PAGE_H
/* Page size when the viewport has room. Falls back to whatever fits. */
const TARGET_SCALE = 0.9
/* Frame around the page. The page slides back as it turns (see `draw`), so
   its silhouette never outgrows the page itself by more than this at
   MAX_TILT — the canvas only needs a hair of margin. */
const FRAME_X = 1.09
const FRAME_Y = 1.03
const MAX_TILT = MathUtils.degToRad(28)
const TOOLBAR_H = 44
const GUTTER = 16
const FOV = 20
const TAN_HALF_FOV = Math.tan(MathUtils.degToRad(FOV / 2))
const MAX_ZOOM = 2.6
const PAPER_THICKNESS = 0.006

type Layout = { w: number; h: number; paperW: number; paperH: number }

function computeLayout(): Layout {
  // 2px for the panel border.
  const availW = window.innerWidth - GUTTER * 2 - 2
  const availH = window.innerHeight - GUTTER * 2 - TOOLBAR_H - 2
  const paperH = Math.floor(Math.min(PAGE_H * TARGET_SCALE, availH / FRAME_Y, availW / FRAME_X / ASPECT))
  const paperW = paperH * ASPECT
  return { w: Math.round(paperW * FRAME_X), h: Math.round(paperH * FRAME_Y), paperW, paperH }
}

type Viewer = {
  resize: (l: Layout) => void
  reset: () => void
  flip: (dir: 1 | -1) => void
  zoomBy: (factor: number) => void
  dispose: () => void
}

type ViewerOpts = {
  layout: Layout
  reducedMotion: boolean
  onReady: () => void
  onFail: () => void
  onZoomChange: (zoomed: boolean) => void
}

/* Off-white back with the print faintly showing through, mirrored the way
   it would be on real paper held up to the light. */
function makeBackTexture(img: CanvasImageSource): CanvasTexture {
  const c = document.createElement('canvas')
  c.width = 1024
  c.height = Math.round(1024 / ASPECT)
  const ctx = c.getContext('2d')!
  ctx.fillStyle = '#f6f4ef'
  ctx.fillRect(0, 0, c.width, c.height)
  ctx.translate(c.width, 0)
  ctx.scale(-1, 1)
  ctx.globalAlpha = 0.06
  ctx.drawImage(img, 0, 0, c.width, c.height)
  const tex = new CanvasTexture(c)
  tex.colorSpace = SRGBColorSpace
  return tex
}

/* Soft contact shadow. shadowBlur on an off-canvas rect is the portable way
   to get a blur (ctx.filter is missing on older Safari). */
function makeShadowTexture(): CanvasTexture {
  const c = document.createElement('canvas')
  c.width = 256
  c.height = 256
  const ctx = c.getContext('2d')!
  const pad = 40
  ctx.shadowColor = 'rgba(0,0,0,1)'
  ctx.shadowBlur = 30
  ctx.shadowOffsetX = 2000
  ctx.fillRect(pad - 2000, pad, c.width - pad * 2, c.height - pad * 2)
  return new CanvasTexture(c)
}

function createViewer(host: HTMLElement, opts: ViewerOpts): Viewer {
  const renderer = new WebGLRenderer({ antialias: true, alpha: true })
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
  renderer.setClearColor(0x000000, 0)
  renderer.autoClear = false
  const canvas = renderer.domElement
  canvas.style.display = 'block'
  canvas.style.touchAction = 'none'
  canvas.style.cursor = 'grab'
  canvas.setAttribute('aria-hidden', 'true')
  host.appendChild(canvas)

  const camera = new PerspectiveCamera(FOV, 1, 0.1, 50)
  const scene = new Scene()
  const shadowScene = new Scene()

  /* Lighting is balanced so the page facing the camera comes out at exactly
     1.0 (Lambert divides by π, hence the π factors) — the PDF's whites stay
     white at rest and only shade once it turns. */
  const lightDir = { x: -0.25, y: 0.3, z: 1 }
  const facing = lightDir.z / Math.hypot(lightDir.x, lightDir.y, lightDir.z)
  const ambient = 0.4
  scene.add(new AmbientLight(0xffffff, ambient * Math.PI))
  const sun = new DirectionalLight(0xffffff, ((1 - ambient) / facing) * Math.PI)
  sun.position.set(lightDir.x, lightDir.y, lightDir.z)
  scene.add(sun)

  const geometry = new BoxGeometry(ASPECT, 1, PAPER_THICKNESS)
  const edgeMat = new MeshLambertMaterial({ color: 0xe6e3dc })
  const frontMat = new MeshLambertMaterial({ color: 0xffffff })
  const backMat = new MeshLambertMaterial({ color: 0xf6f4ef })
  const paper = new Mesh(geometry, [edgeMat, edgeMat, edgeMat, edgeMat, frontMat, backMat])
  paper.rotation.order = 'YXZ'
  scene.add(paper)

  const shadowTex = makeShadowTexture()
  const shadowGeo = new PlaneGeometry(ASPECT * 1.18, 1.1)
  const shadowMat = new MeshBasicMaterial({
    map: shadowTex,
    color: 0x000000,
    transparent: true,
    opacity: 0.35,
    depthWrite: false,
  })
  const shadow = new Mesh(shadowGeo, shadowMat)
  shadowScene.add(shadow)

  const textures: Texture[] = [shadowTex]
  let ready = false
  let disposed = false

  /* ─── State ─── */
  const reduced = opts.reducedMotion
  const s = {
    spin: reduced ? 0 : -1.1,
    spinVel: 0,
    spinTarget: 0 as number | null,
    tilt: reduced ? 0 : 0.16,
    tiltVel: 0,
    zoom: 1,
    zoomT: 1,
    camX: 0,
    camY: 0,
    camXT: 0,
    camYT: 0,
    dragging: false,
  }
  let layout = opts.layout
  let baseHalfH = 0.5
  let lastZoomed = false

  const nearestFace = (spin: number) => Math.round(spin / Math.PI) * Math.PI

  function clampCam(x: number, y: number, zoom: number): [number, number] {
    const hh = baseHalfH / zoom
    const hw = hh * camera.aspect
    const limX = Math.max(0, ASPECT / 2 + 0.03 - hw)
    const limY = Math.max(0, 0.5 + 0.03 - hh)
    return [MathUtils.clamp(x, -limX, limX), MathUtils.clamp(y, -limY, limY)]
  }

  function reportZoom() {
    const zoomed = s.zoomT > 1.01
    if (zoomed !== lastZoomed) {
      lastZoomed = zoomed
      canvas.style.cursor = zoomed ? 'move' : 'grab'
      opts.onZoomChange(zoomed)
    }
  }

  /* Zoom so the point under (clientX, clientY) stays put. */
  function zoomAt(clientX: number, clientY: number, next: number, immediate = false) {
    const rect = canvas.getBoundingClientRect()
    const nx = ((clientX - rect.left) / rect.width) * 2 - 1
    const ny = -(((clientY - rect.top) / rect.height) * 2 - 1)
    const z0 = s.zoomT
    const z1 = MathUtils.clamp(next, 1, MAX_ZOOM)
    const hh0 = baseHalfH / z0
    const hh1 = baseHalfH / z1
    const wx = s.camXT + nx * hh0 * camera.aspect
    const wy = s.camYT + ny * hh0
    ;[s.camXT, s.camYT] = z1 <= 1.001 ? [0, 0] : clampCam(wx - nx * hh1 * camera.aspect, wy - ny * hh1, z1)
    s.zoomT = z1
    if (immediate) {
      s.zoom = z1
      s.camX = s.camXT
      s.camY = s.camYT
    }
    // Reading up close only makes sense square-on.
    if (z1 > 1.001 && !s.dragging) s.spinTarget = nearestFace(s.spin)
    reportZoom()
    kick()
  }

  function spring(x: number, v: number, target: number, k: number, zeta: number, dt: number): [number, number] {
    const c = 2 * Math.sqrt(k) * zeta
    const nv = v + (-k * (x - target) - c * v) * dt
    return [x + nv * dt, nv]
  }

  /* Advance one frame; returns whether anything is still moving. */
  function step(dt: number): boolean {
    const zeta = reduced ? 1 : 0.72
    if (!s.dragging) {
      if (s.spinTarget === null) {
        s.spinVel *= Math.exp(-2.5 * dt)
        s.spin += s.spinVel * dt
        // Once the throw runs out of steam, settle onto whichever face it's
        // heading for so the page always ends up readable.
        if (Math.abs(s.spinVel) < 3) s.spinTarget = nearestFace(s.spin + s.spinVel * 0.2)
      } else {
        ;[s.spin, s.spinVel] = spring(s.spin, s.spinVel, s.spinTarget, 70, zeta, dt)
      }
      ;[s.tilt, s.tiltVel] = spring(s.tilt, s.tiltVel, 0, 70, zeta, dt)
    }
    const ease = 1 - Math.exp(-dt * 18)
    s.zoom += (s.zoomT - s.zoom) * ease
    s.camX += (s.camXT - s.camX) * ease
    s.camY += (s.camYT - s.camY) * ease

    const settled =
      !s.dragging &&
      s.spinTarget !== null &&
      Math.abs(s.spin - s.spinTarget) < 5e-4 &&
      Math.abs(s.spinVel) < 5e-3 &&
      Math.abs(s.tilt) < 5e-4 &&
      Math.abs(s.tiltVel) < 5e-3 &&
      Math.abs(s.zoom - s.zoomT) < 1e-4 &&
      Math.abs(s.camX - s.camXT) < 1e-5 &&
      Math.abs(s.camY - s.camYT) < 1e-5
    if (settled) {
      // Land exactly square so the texture samples crisply at rest.
      s.spin = s.spinTarget!
      s.spinVel = s.tilt = s.tiltVel = 0
      s.zoom = s.zoomT
      s.camX = s.camXT
      s.camY = s.camYT
    }
    return !settled
  }

  function draw() {
    if (!ready) return
    paper.rotation.set(s.tilt, s.spin, 0)
    // Slide the page back by however far its nearest edge swings forward, so
    // perspective never pushes it past the frame.
    const push = (ASPECT / 2) * Math.abs(Math.sin(s.spin)) * Math.cos(s.tilt) + 0.5 * Math.abs(Math.sin(s.tilt))
    paper.position.z = -push
    shadow.position.set(0, -0.012, -push - 0.12)
    shadow.scale.set(Math.max(Math.abs(Math.cos(s.spin)), 0.04), Math.max(Math.cos(s.tilt), 0.3), 1)
    camera.position.set(s.camX, s.camY, baseHalfH / TAN_HALF_FOV / s.zoom)
    renderer.clear()
    renderer.render(shadowScene, camera)
    renderer.render(scene, camera)
  }

  let raf = 0
  let last = 0
  function frame(now: number) {
    raf = 0
    const dt = Math.min((now - last) / 1000, 1 / 30)
    last = now
    const moving = step(dt)
    draw()
    if (moving) raf = requestAnimationFrame(frame)
  }
  function kick() {
    if (disposed || raf) return
    last = performance.now()
    raf = requestAnimationFrame(frame)
  }

  function resize(l: Layout) {
    layout = l
    renderer.setSize(l.w, l.h)
    camera.aspect = l.w / l.h
    camera.updateProjectionMatrix()
    baseHalfH = l.h / l.paperH / 2
    ;[s.camXT, s.camYT] = clampCam(s.camXT, s.camYT, s.zoomT)
    s.camX = s.camXT
    s.camY = s.camYT
    draw()
    kick()
  }
  resize(layout)

  /* ─── Texture ─── */
  new TextureLoader().load(
    RESUME_IMG,
    tex => {
      if (disposed) {
        tex.dispose()
        return
      }
      tex.colorSpace = SRGBColorSpace
      tex.anisotropy = renderer.capabilities.getMaxAnisotropy()
      frontMat.map = tex
      frontMat.needsUpdate = true
      const back = makeBackTexture(tex.image as CanvasImageSource)
      backMat.map = back
      backMat.color.set(0xffffff)
      backMat.needsUpdate = true
      textures.push(tex, back)
      ready = true
      opts.onReady()
      last = performance.now()
      kick()
    },
    undefined,
    () => !disposed && opts.onFail(),
  )

  /* ─── Input ─── */
  type Mode = 'rotate' | 'pan' | 'pinch'
  const pointers = new Map<number, { x: number; y: number }>()
  let mode: Mode = 'rotate'
  let lastMoveAt = 0
  let pinch = { dist: 1, zoom: 1, x: 0, y: 0 }

  const pinchInfo = () => {
    const [a, b] = [...pointers.values()]
    return { dist: Math.hypot(a.x - b.x, a.y - b.y) || 1, x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 }
  }

  function startGesture() {
    if (pointers.size >= 2) {
      mode = 'pinch'
      const p = pinchInfo()
      pinch = { ...p, zoom: s.zoomT }
    } else {
      mode = s.zoomT > 1.01 ? 'pan' : 'rotate'
      if (mode === 'rotate') {
        s.spinTarget = null
        s.spinVel = 0
      }
    }
  }

  function onPointerDown(e: PointerEvent) {
    if (e.button !== 0 && e.pointerType === 'mouse') return
    canvas.setPointerCapture(e.pointerId)
    pointers.set(e.pointerId, { x: e.clientX, y: e.clientY })
    s.dragging = true
    if (mode !== 'pinch' || pointers.size >= 2) startGesture()
    canvas.style.cursor = mode === 'rotate' ? 'grabbing' : 'move'
    lastMoveAt = performance.now()
    kick()
  }

  function onPointerMove(e: PointerEvent) {
    const prev = pointers.get(e.pointerId)
    if (!prev) return
    const dx = e.clientX - prev.x
    const dy = e.clientY - prev.y
    pointers.set(e.pointerId, { x: e.clientX, y: e.clientY })
    const now = performance.now()
    const moveDt = Math.max((now - lastMoveAt) / 1000, 1 / 240)
    lastMoveAt = now

    if (mode === 'pinch' && pointers.size >= 2) {
      const p = pinchInfo()
      // Pan with the midpoint, then zoom about it.
      const wpp = (baseHalfH * 2) / s.zoomT / layout.h
      ;[s.camXT, s.camYT] = clampCam(s.camXT - (p.x - pinch.x) * wpp, s.camYT + (p.y - pinch.y) * wpp, s.zoomT)
      pinch.x = p.x
      pinch.y = p.y
      zoomAt(p.x, p.y, (pinch.zoom * p.dist) / pinch.dist, true)
    } else if (mode === 'pan') {
      const wpp = (baseHalfH * 2) / s.zoomT / layout.h
      ;[s.camXT, s.camYT] = clampCam(s.camXT - dx * wpp, s.camYT + dy * wpp, s.zoomT)
      s.camX = s.camXT
      s.camY = s.camYT
    } else if (mode === 'rotate') {
      const ds = dx * (Math.PI / layout.w)
      s.spin += ds
      s.spinVel = s.spinVel * 0.4 + (ds / moveDt) * 0.6
      s.tilt = MathUtils.clamp(s.tilt + dy * (Math.PI / layout.h) * 0.6, -MAX_TILT, MAX_TILT)
    }
    kick()
  }

  function onPointerUp(e: PointerEvent) {
    if (!pointers.delete(e.pointerId)) return
    if (pointers.size === 0) {
      s.dragging = false
      // A drag that stopped before letting go shouldn't fling.
      if (mode === 'rotate' && performance.now() - lastMoveAt > 80) s.spinVel = 0
      if (s.zoomT > 1.01) s.spinTarget = nearestFace(s.spin)
      mode = 'rotate'
      canvas.style.cursor = s.zoomT > 1.01 ? 'move' : 'grab'
    } else {
      // Lifting one finger of a pinch carries on as a one-finger gesture.
      startGesture()
    }
    kick()
  }

  function onWheel(e: WheelEvent) {
    e.preventDefault()
    let dy = e.deltaY
    if (e.deltaMode === 1) dy *= 16
    // ctrlKey is how trackpad pinches arrive; those deltas are much smaller.
    const k = e.ctrlKey ? 0.012 : 0.0018
    zoomAt(e.clientX, e.clientY, s.zoomT * Math.exp(-dy * k))
  }

  function onDblClick(e: MouseEvent) {
    zoomAt(e.clientX, e.clientY, s.zoomT > 1.05 ? 1 : 2.2)
  }

  function onContextLost(e: Event) {
    e.preventDefault()
    opts.onFail()
  }

  canvas.addEventListener('pointerdown', onPointerDown)
  canvas.addEventListener('pointermove', onPointerMove)
  canvas.addEventListener('pointerup', onPointerUp)
  canvas.addEventListener('pointercancel', onPointerUp)
  canvas.addEventListener('wheel', onWheel, { passive: false })
  canvas.addEventListener('dblclick', onDblClick)
  canvas.addEventListener('webglcontextlost', onContextLost)

  return {
    resize,
    reset() {
      s.spinTarget = Math.round(s.spin / (Math.PI * 2)) * Math.PI * 2
      s.zoomT = 1
      s.camXT = s.camYT = 0
      reportZoom()
      kick()
    },
    flip(dir) {
      s.spinTarget = nearestFace(s.spinTarget ?? s.spin) + dir * Math.PI
      s.zoomT = 1
      s.camXT = s.camYT = 0
      reportZoom()
      kick()
    },
    zoomBy(factor) {
      const rect = canvas.getBoundingClientRect()
      zoomAt(rect.left + rect.width / 2, rect.top + rect.height / 2, s.zoomT * factor)
    },
    dispose() {
      disposed = true
      cancelAnimationFrame(raf)
      canvas.removeEventListener('pointerdown', onPointerDown)
      canvas.removeEventListener('pointermove', onPointerMove)
      canvas.removeEventListener('pointerup', onPointerUp)
      canvas.removeEventListener('pointercancel', onPointerUp)
      canvas.removeEventListener('wheel', onWheel)
      canvas.removeEventListener('dblclick', onDblClick)
      canvas.removeEventListener('webglcontextlost', onContextLost)
      textures.forEach(t => t.dispose())
      geometry.dispose()
      shadowGeo.dispose()
      ;[edgeMat, frontMat, backMat, shadowMat].forEach(m => m.dispose())
      renderer.dispose()
      renderer.forceContextLoss()
      canvas.remove()
    },
  }
}

const FOCUSABLE = 'a[href], button:not([disabled]), [tabindex]:not([tabindex="-1"])'

const iconBtn =
  'hover-accent pressable inline-flex items-center justify-center w-8 h-8 shrink-0'

export default function ResumeModal({ onClose }: { onClose: () => void }) {
  const [layout, setLayout] = useState(computeLayout)
  const [status, setStatus] = useState<'loading' | 'ready' | 'fallback'>('loading')
  const [zoomed, setZoomed] = useState(false)
  const [closing, setClosing] = useState(false)
  const [coarse] = useState(() => window.matchMedia('(pointer: coarse)').matches)
  const [reducedMotion] = useState(() => window.matchMedia('(prefers-reduced-motion: reduce)').matches)
  const hostRef = useRef<HTMLDivElement>(null)
  const dialogRef = useRef<HTMLDivElement>(null)
  const closeRef = useRef<HTMLButtonElement>(null)
  const viewerRef = useRef<Viewer | null>(null)

  const requestClose = useCallback(() => {
    if (reducedMotion) return onClose()
    setClosing(true)
    window.setTimeout(onClose, 160)
  }, [onClose, reducedMotion])

  useEffect(() => {
    const host = hostRef.current
    if (!host) return
    let viewer: Viewer
    try {
      viewer = createViewer(host, {
        layout: computeLayout(),
        reducedMotion,
        onReady: () => setStatus('ready'),
        onFail: () => setStatus('fallback'),
        onZoomChange: setZoomed,
      })
    } catch {
      // No WebGL — the flat image below takes over.
      queueMicrotask(() => setStatus('fallback'))
      return
    }
    viewerRef.current = viewer
    return () => {
      viewer.dispose()
      viewerRef.current = null
    }
  }, [reducedMotion])

  useEffect(() => {
    viewerRef.current?.resize(layout)
  }, [layout])

  useEffect(() => {
    const onResize = () => setLayout(computeLayout())
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])

  // Lock page scroll and hand focus back to the trigger on close.
  useEffect(() => {
    const prevFocus = document.activeElement as HTMLElement | null
    const html = document.documentElement
    const prevOverflow = html.style.overflow
    html.style.overflow = 'hidden'
    closeRef.current?.focus({ preventScroll: true })
    return () => {
      html.style.overflow = prevOverflow
      prevFocus?.focus({ preventScroll: true })
    }
  }, [])

  function onKeyDown(e: ReactKeyboardEvent) {
    const v = viewerRef.current
    switch (e.key) {
      case 'Escape':
        e.preventDefault()
        requestClose()
        return
      case 'ArrowLeft':
        v?.flip(-1)
        break
      case 'ArrowRight':
        v?.flip(1)
        break
      case '+':
      case '=':
        v?.zoomBy(1.4)
        break
      case '-':
        v?.zoomBy(1 / 1.4)
        break
      case '0':
        v?.reset()
        break
      case 'Tab': {
        // Keep focus inside the dialog.
        const nodes = dialogRef.current?.querySelectorAll<HTMLElement>(FOCUSABLE)
        if (!nodes?.length) return
        const first = nodes[0]
        const lastNode = nodes[nodes.length - 1]
        if (e.shiftKey && document.activeElement === first) {
          lastNode.focus()
        } else if (!e.shiftKey && document.activeElement === lastNode) {
          first.focus()
        } else return
        break
      }
      default:
        return
    }
    e.preventDefault()
  }

  const hint = zoomed
    ? coarse
      ? 'drag to pan · pinch out to zoom out'
      : 'drag to pan · double-click to zoom out'
    : coarse
      ? 'drag to rotate · pinch to zoom'
      : 'drag to rotate · scroll to zoom'

  return createPortal(
    <div
      className="modal-backdrop fixed inset-0 z-[2000] flex items-center justify-center"
      data-closing={closing || undefined}
      onPointerDown={e => {
        if (e.target === e.currentTarget) requestClose()
      }}
      onKeyDown={onKeyDown}
    >
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label="Resume viewer"
        tabIndex={-1}
        className="modal-panel flex flex-col border overflow-hidden"
        style={{ width: layout.w + 2, borderColor: 'var(--border)', background: 'var(--bg2)' }}
      >
        <div ref={hostRef} className="relative select-none" style={{ width: layout.w, height: layout.h }}>
          {status === 'loading' && (
            <div
              className="absolute inset-0 flex items-center justify-center text-[12px] tracking-wider"
              style={{ color: 'var(--dim)' }}
            >
              loading resume…
            </div>
          )}
          {status === 'fallback' && (
            <div className="absolute inset-0 flex items-center justify-center">
              <img
                src={RESUME_IMG}
                alt="Arpit Mishra's resume"
                style={{ width: layout.paperW, height: layout.paperH }}
              />
            </div>
          )}
        </div>
        <div
          className="flex items-center justify-between gap-3 border-t pl-4 pr-2"
          style={{ height: TOOLBAR_H, borderColor: 'var(--border)' }}
        >
          <div className="flex items-baseline gap-3 min-w-0 text-[12px] tracking-wider">
            <span style={{ color: 'var(--fg)' }}>resume.pdf</span>
            {status === 'ready' && (
              <span className="truncate hidden sm:inline" style={{ color: 'var(--dim)', opacity: 0.7 }} aria-hidden="true">
                {hint}
              </span>
            )}
          </div>
          <div className="flex items-center gap-1">
            {status === 'ready' && (
              <button type="button" className={iconBtn} onClick={() => viewerRef.current?.reset()} aria-label="Reset view" title="Reset view (0)">
                <svg viewBox="0 0 24 24" className="w-4 h-4 fill-none stroke-current" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M3 12a9 9 0 1 0 3-6.7" />
                  <path d="M3 4v5h5" />
                </svg>
              </button>
            )}
            <a
              href={RESUME_PDF}
              download="Arpit-Mishra-Resume.pdf"
              className={iconBtn}
              aria-label="Download PDF"
              title="Download PDF"
            >
              <svg viewBox="0 0 24 24" className="w-4 h-4 fill-none stroke-current" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
                <path d="M12 4v11m0 0-4.5-4.5M12 15l4.5-4.5M5 20h14" />
              </svg>
            </a>
            <button ref={closeRef} type="button" className={iconBtn} onClick={requestClose} aria-label="Close" title="Close (Esc)">
              <svg viewBox="0 0 24 24" className="w-4 h-4 fill-none stroke-current" strokeWidth="1.6" strokeLinecap="round">
                <path d="M6 6l12 12M18 6 6 18" />
              </svg>
            </button>
          </div>
        </div>
      </div>
    </div>,
    document.body,
  )
}
