import { interpolate, useCurrentFrame, useVideoConfig } from 'remotion'
import { TerminalFrame } from '../components/TerminalFrame'
import { CodeLine } from '../components/CodeLine'
import { fonts } from '../lib/fonts'

const PROMPT = '~/projects'
const CHARS_PER_FRAME = 1.1

type Props = { baseStart: number }

export const Scene4_NewTerminal: React.FC<Props> = ({ baseStart }) => {
  const frame = useCurrentFrame()
  const { fps } = useVideoConfig()
  const local = frame - baseStart
  const slideUp = interpolate(local, [0, 24], [80, 0], { extrapolateRight: 'clamp' })
  const fadeIn = interpolate(local, [0, 14], [0, 1], { extrapolateRight: 'clamp' })
  const command = '$ skill-organizer skill add helpful-formatter --check-security'
  const cmdChars = Math.min(command.length, Math.floor(local * CHARS_PER_FRAME))
  return (
    <div
      style={{
        position: 'absolute',
        inset: 0,
        background: '#020611',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 80,
        opacity: fadeIn,
        transform: `translateY(${slideUp}px)`,
      }}
    >
      <TerminalFrame
        title="skill-organizer — ~/projects"
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
                {command.slice(0, cmdChars)}
              </span>
              <span
                style={{
                  display: 'inline-block',
                  width: 10,
                  height: 18,
                  background:
                    Math.floor(frame / (fps * 0.5)) % 2 === 0
                      ? 'rgba(67, 238, 255, 0.92)'
                      : 'transparent',
                  verticalAlign: 'middle',
                }}
              />
            </div>
            <CodeLine
              line={[
                { text: '→ ', color: 'cyan' as const, bold: true },
                { text: 'running security check before linking', color: 'default' },
              ]}
              start={Math.max(0, command.length / CHARS_PER_FRAME + 4)}
              cps={50}
            />
            <CodeLine
              line={[
                { text: '✓ ', color: 'green' as const },
                { text: 'added ', color: 'muted' },
                { text: 'helpful-formatter', color: 'cyan', bold: true },
                { text: ' (low risk) to ', color: 'muted' },
                { text: '~/.agents', color: 'default' },
              ]}
              start={Math.max(0, command.length / CHARS_PER_FRAME + 22)}
              cps={50}
            />
          </div>
        }
      />
    </div>
  )
}
