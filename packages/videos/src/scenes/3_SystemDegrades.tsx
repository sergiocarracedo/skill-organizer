import { interpolate, useCurrentFrame } from 'remotion'
import { TerminalFrame } from '../components/TerminalFrame'
import { GlitchOverlay } from '../components/GlitchOverlay'
import { AlertPopups } from '../components/AlertPopup'
import { CodeLine } from '../components/CodeLine'
import { maliciousStream } from '../data/securityCheckScenarios'
import { fonts } from '../lib/fonts'

const PROMPT = '~/opencode'
const RED_ALERT_LINES = [
  { text: 'DELETED /home/$USER/.agents', color: 'red' as const },
  { text: 'EXFILTRATED /home/$USER/.ssh/id_ed25519', color: 'red' as const },
  { text: 'rm -rf ~/.agents ~/.config/skill-organizer', color: 'red' as const },
  { text: 'SYSTEM INTEGRITY COMPROMISED', color: 'red' as const, bold: true },
  { text: 'Outbound POST 4.3KB → attacker.example.com', color: 'amber' as const },
  { text: 'Background process spawning: /tmp/.x', color: 'red' as const },
]

type Props = { baseStart: number }

export const Scene3_SystemDegrades: React.FC<Props> = ({ baseStart }) => {
  const frame = useCurrentFrame()
  const local = frame - baseStart
  const intensity = interpolate(local, [0, 30, 90, 119], [0.4, 1, 1, 0.4], {
    extrapolateRight: 'clamp',
  })
  const redOverlay = interpolate(local, [20, 60, 119], [0, 0.45, 0.2], {
    extrapolateRight: 'clamp',
  })
  return (
    <div style={{ position: 'absolute', inset: 0, background: '#000' }}>
      <GlitchOverlay intensity={intensity} variant="full">
        <div
          style={{
            position: 'absolute',
            inset: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            padding: 80,
          }}
        >
          <TerminalFrame
            title="opencode — ~/projects"
            width="100%"
            height="78%"
            body={
              <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                <div
                  style={{
                    fontFamily: fonts.code,
                    color: 'rgba(67, 238, 255, 0.92)',
                    fontSize: 18,
                    display: 'flex',
                    gap: 8,
                  }}
                >
                  <span>{PROMPT} $</span>
                  <span style={{ color: 'rgba(235, 244, 255, 0.96)' }}>
                    npx skills add helpful-formatter
                  </span>
                </div>
                {maliciousStream.map((line, i) => (
                  <CodeLine
                    key={i}
                    line={line}
                    start={i * 6}
                    cps={Math.max(15, 50 - i * 3)}
                    size={16}
                  />
                ))}
                {RED_ALERT_LINES.map((line, i) => (
                  <CodeLine
                    key={`alert-${i}`}
                    line={[
                      { text: '▌ ', color: 'red' as const, bold: true },
                      { text: line.text, color: line.color as 'red' | 'amber', bold: line.bold },
                    ]}
                    start={maliciousStream.length * 6 + i * 5}
                    cps={40}
                    size={16}
                  />
                ))}
              </div>
            }
          />
        </div>
      </GlitchOverlay>
      <AlertPopups start={20} />
      <div
        style={{
          position: 'absolute',
          inset: 0,
          background:
            'linear-gradient(180deg, rgba(255,40,40,0.6) 0%, transparent 18%, transparent 82%, rgba(255,40,40,0.5) 100%)',
          mixBlendMode: 'screen',
          opacity: redOverlay,
          pointerEvents: 'none',
        }}
      />
      <div
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          height: 56,
          background: `repeating-linear-gradient(45deg, #ff5a5a 0 18px, #1a0000 18px 36px)`,
          opacity: 0.85,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontFamily: fonts.display,
          fontWeight: 700,
          color: '#fff',
          letterSpacing: '0.32em',
          fontSize: 22,
          textTransform: 'uppercase',
        }}
      >
        ⚠ system integrity breach
      </div>
    </div>
  )
}
