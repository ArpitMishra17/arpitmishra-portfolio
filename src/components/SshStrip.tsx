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
      <span className="hidden sm:inline" style={{ color: 'var(--dim)' }}>
        <span style={{ color: 'var(--accent)' }}>//</span> prefer a terminal? run
      </span>
      <SshHint tipSide="below" compact />
    </div>
  )
}
