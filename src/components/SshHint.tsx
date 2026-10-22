import { useEffect, useId, useRef, useState } from 'react'

export const SSH_COMMAND = 'ssh tui.arpitmishra.dev'

interface SshHintProps {
  /** Which side of the command the "?" tooltip opens on. */
  tipSide?: 'above' | 'below'
  /** Hide the "copy" label on narrow screens to save room. */
  compact?: boolean
}

/**
 * The ssh command as a copy button, with a "?" that explains how to run it.
 * The tooltip opens on hover and keyboard focus (desktop) and toggles on tap
 * (touch). Esc, an outside click, or scrolling closes it.
 */
export default function SshHint({ tipSide = 'above', compact = false }: SshHintProps) {
  const [copied, setCopied] = useState(false)
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLSpanElement>(null)
  const tipId = useId()

  useEffect(() => {
    if (!copied) return
    const t = window.setTimeout(() => setCopied(false), 1400)
    return () => window.clearTimeout(t)
  }, [copied])

  useEffect(() => {
    if (!open) return
    const close = () => setOpen(false)
    const onPointerDown = (e: PointerEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) close()
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') close()
    }
    document.addEventListener('pointerdown', onPointerDown)
    document.addEventListener('keydown', onKey)
    window.addEventListener('scroll', close, { passive: true })
    return () => {
      document.removeEventListener('pointerdown', onPointerDown)
      document.removeEventListener('keydown', onKey)
      window.removeEventListener('scroll', close)
    }
  }, [open])

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(SSH_COMMAND)
      setCopied(true)
    } catch {
      // Clipboard needs a secure context + permission; the command is still
      // visible and selectable, so failing quietly is fine.
    }
  }

  return (
    <span ref={rootRef} className="ssh-hint">
      <button type="button" onClick={copy} className="ssh-cmd pressable" aria-label={`Copy command: ${SSH_COMMAND}`}>
        <span aria-hidden="true" style={{ color: 'var(--accent)' }}>$</span>
        <span>{SSH_COMMAND}</span>
        <span
          className={`ssh-copy${compact ? ' hidden sm:inline' : ''}`}
          data-done={copied}
          aria-live="polite"
        >
          {copied ? 'copied' : 'copy'}
        </span>
      </button>

      <span className="ssh-help" data-open={open}>
        <button
          type="button"
          className="ssh-q pressable"
          aria-label="How do I use this?"
          aria-expanded={open}
          aria-describedby={tipId}
          onClick={() => setOpen(o => !o)}
        >
          ?
        </button>
        <span id={tipId} role="tooltip" className="ssh-tip" data-side={tipSide}>
          <b>How to run it</b>
          <ol>
            <li>Open a terminal (Ghostty, Alacritty, Warp, etc.)</li>
            <li>Paste the command and press <kbd>Enter</kbd></li>
            <li>
              When it asks "Are you sure you want to continue connecting?", type <kbd>yes</kbd> and
              press <kbd>Enter</kbd>
            </li>
          </ol>
        </span>
      </span>
    </span>
  )
}
