import { interpolate, useCurrentFrame, useVideoConfig } from 'remotion'
import { TerminalFrame } from '../components/TerminalFrame'
import { CodeLine } from '../components/CodeLine'
import { maliciousStream } from '../data/securityCheckScenarios'
import { fonts } from '../lib/fonts'

const PROMPT = '~/opencode'
const CHARS_PER_FRAME = 0.9

type Props = { baseStart: number }

export const Scene1_OpencodeLoads: React.FC<Props> = ({ baseStart }) => {
  const frame = useCurrentFrame()
  const { fps } = useVideoConfig()
  const local = frame - baseStart
  const fadeIn = interpolate(local, [0, 12], [0, 1], { extrapolateRight: 'clamp' })

  const promptTime = 0
  const command = '$ npx skills add helpful-formatter'
  const commandChars = Math.min(command.length, Math.floor((local - promptTime) * CHARS_PER_FRAME))
  const commandVisible = command.slice(0, commandChars)
  return (
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
              <span style={{ color: 'rgba(67, 238, 255, 0.92)' }}>{PROMPT} $</span>
              <span style={{ color: 'rgba(235, 244, 255, 0.96)' }}>{commandVisible}</span>
              <Cursor frame={frame - promptTime} fps={fps} />
            </div>
            {maliciousStream.slice(0, 3).map((line, i) => (
              <CodeLine key={i} line={line} start={promptTime + 12 + i * 6} cps={50} />
            ))}
          </div>
        }
      />
    </div>
  )
}

const Cursor: React.FC<{ frame: number; fps: number }> = ({ frame, fps }) => {
  if (frame < 0) return null
  const blink = Math.floor(frame / (fps * 0.5)) % 2 === 0
  return (
    <span
      style={{
        display: 'inline-block',
        width: 10,
        height: 18,
        background: blink ? 'rgba(67, 238, 255, 0.92)' : 'transparent',
        verticalAlign: 'middle',
      }}
    />
  )
}
