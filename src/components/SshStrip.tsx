import SshHint from './SshHint'

interface SshStripProps {
  visible: boolean
}

/**
 * Slim announcement strip pinned above the nav. It slides away once the page
 * scrolls (the nav drops back to the top in step), and comes back at the top.
 */
export default function SshStrip({ visible }: SshStripProps) {
  return (
    <div className="ssh-strip" data-visible={visible} inert={!visible}>
      <span className="ssh-anchor">
        <SshHint tipSide="below" compact />
        <span className="ssh-note" aria-hidden="true">
          <svg viewBox="0 0 34 44" width="34" height="44" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path className="ssh-note-shaft" pathLength="1" d="M26 42 C 30 24, 8 26, 12 5" />
            <path className="ssh-note-head" pathLength="1" d="M5 12 L12 4 L19 11" />
          </svg>
          <span>psst, try it in your terminal!</span>
        </span>
      </span>
    </div>
  )
}
