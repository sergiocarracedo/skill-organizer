import { interpolate, useCurrentFrame } from 'remotion'
import { TerminalFrame } from '../components/TerminalFrame'
import { CodeLine } from '../components/CodeLine'
import { GlitchOverlay } from '../components/GlitchOverlay'
import { maliciousStream } from '../data/securityCheckScenarios'
import { fonts } from '../lib/fonts'

const PROMPT = '~/opencode'

type Props = { baseStart: number }

export const Scene2_MaliciousCode: React.FC<Props> = ({ baseStart }) => {
  const frame = useCurrentFrame()
  const local = frame - baseStart
  const fadeIn = interpolate(local, [0, 6], [0, 1], { extrapolateRight: 'clamp' })
  const intensity = interpolate(local, [0, 90, 150], [0, 0.6, 0.6], {
    extrapolateRight: 'clamp',
  })

  return (
    <GlitchOverlay intensity={intensity} variant="rgb-split">
      <div
        style={{
          position: 'absolute',
          inset: 0,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          padding: 80,
          opacity: fadeIn,
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
                <span style={{ opacity: 0.6 }}>▮</span>
              </div>
              {maliciousStream.map((line, i) => (
                <CodeLine key={i} line={line} start={i * 10} cps={Math.max(20, 50 - i * 3)} />
              ))}
            </div>
          }
        />
      </div>
    </GlitchOverlay>
  )
}
