const recentTracks = [
  { name: 'Midnight City', artist: 'M83', time: '4:03' },
  { name: 'A Real Hero', artist: 'College & Electric Youth', time: '4:25' },
  { name: 'Nightcall', artist: 'Kavinsky', time: '4:16' },
  { name: 'Redbone', artist: 'Childish Gambino', time: '5:26' },
]

export default function Spotify() {
  return (
    <div className="mb-10">
      <div
        className="text-[11px] tracking-[2.5px] uppercase mb-6 flex justify-between items-center"
        style={{ color: 'var(--dim)' }}
      >
        <span>What I am listening to...(still in development)</span>
      </div>

      <div
        className="grid grid-cols-1 md:grid-cols-2"
        style={{ gap: '1px', background: 'var(--border)', border: '1px solid var(--border)' }}
      >
        {/* Now Playing */}
        <div className="flex gap-4 items-center p-6" style={{ background: 'var(--bg2)' }}>
          <div className="shrink-0 flex items-end gap-0.5 h-8">
            {[60, 100, 40, 80, 50].map((h, i) => (
              <div
                key={i}
                className="w-[3px] rounded-[1px]"
                style={{
                  background: 'var(--green)',
                  height: `${h}%`,
                  animation: `eqBar 1.2s ease-in-out infinite`,
                  animationDelay: `${[0, 0.15, 0.3, 0.45, 0.1][i]}s`,
                }}
              />
            ))}
          </div>
          <div className="flex-1 min-w-0">
            <div
              className="text-[11px] tracking-[1.5px] uppercase mb-1 flex items-center gap-1.5"
              style={{ color: 'var(--green)' }}
            >
              <span className="text-[13px]">♫</span> Now Playing
            </div>
            <div
              className="text-[15px] whitespace-nowrap overflow-hidden text-ellipsis"
              style={{ color: 'var(--fg)' }}
            >
              Resonance
            </div>
            <div
              className="text-[13px] whitespace-nowrap overflow-hidden text-ellipsis"
              style={{ color: 'var(--dim)' }}
            >
              Home
            </div>
            <div
              className="text-[12px] mt-0.5 whitespace-nowrap overflow-hidden text-ellipsis opacity-50"
              style={{ color: 'var(--dim)' }}
            >
              Odyssey
            </div>
            <div
              className="h-0.5 mt-2 rounded-[1px] overflow-hidden"
              style={{ background: 'var(--border)' }}
            >
              <div
                className="h-full rounded-[1px]"
                style={{
                  background: 'var(--green)',
                  animation: 'progressScrub 12s linear infinite',
                }}
              />
            </div>
          </div>
        </div>

        {/* Recent Tracks */}
        <div className="p-6" style={{ background: 'var(--bg2)' }}>
          <div
            className="text-[11px] tracking-[1.5px] uppercase mb-3"
            style={{ color: 'var(--dim)' }}
          >
            Recently Played
          </div>
          {recentTracks.map((track, i) => (
            <div
              key={i}
              className="flex justify-between items-center py-2 text-[13px] transition-all hover:pl-1.5"
              style={{
                borderBottom: i < recentTracks.length - 1 ? '1px solid var(--border)' : 'none',
              }}
            >
              <div>
                <div style={{ color: 'var(--fg)' }}>{track.name}</div>
                <div className="text-[12px]" style={{ color: 'var(--dim)' }}>
                  {track.artist}
                </div>
              </div>
              <div className="text-[12px] shrink-0" style={{ color: 'var(--dim)' }}>
                {track.time}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
