import { interpolate, useCurrentFrame } from 'remotion'
import { TerminalFrame } from '../components/TerminalFrame'
import { CodeLine } from '../components/CodeLine'
import { fonts } from '../lib/fonts'
import { colors } from '../lib/colors'

const PROMPT = '~/projects'
const CHARS_PER_FRAME = 1.0

const safe = [
  { text: '• ', color: 'default' },
  { text: 'helpful-formatter', color: 'green' as const, bold: true },
  { text: ' - Score: ', color: 'default' },
  { text: '5', color: 'green' },
  { text: ' │ Reads stdin, writes stdout', color: 'muted' },
]
const dangerous = [
  { text: '• ', color: 'default' },
  { text: 'timebomb', color: 'red' as const, bold: true },
  { text: ' - Score: ', color: 'default' },
  { text: '88', color: 'red' as const, bold: true },
  { text: ' │ Date-gated payload', color: 'muted' },
]

type Props = { baseStart: number }

export const Scene5_CheckSecurity: React.FC<Props> = ({ baseStart }) => {
  const frame = useCurrentFrame()
  const local = frame - baseStart
  const fadeIn = interpolate(local, [0, 6], [0, 1], { extrapolateRight: 'clamp' })

  const cmd = '$ skill-organizer skill check-security'
  const cmdChars = Math.min(cmd.length, Math.floor(local * CHARS_PER_FRAME))
  const progressStart = Math.max(0, cmd.length / CHARS_PER_FRAME + 4)
  const safeStart = progressStart + 24
  const dangerStart = safeStart + 4
  const totalStart = dangerStart + 4
  const questionStart = totalStart + 8
  const answerStart = questionStart + 20
  const successStart = answerStart + 24
  const showQuestion = local >= questionStart
  const showAnswer = local >= answerStart
  const showSuccess = local >= successStart

  const cursorShown = showQuestion && !showAnswer
  const questionElapsed = local - questionStart
  const blink = showQuestion && Math.floor(questionElapsed / 12) % 2 === 0

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
              <span style={{ color: 'rgba(235, 244, 255, 0.96)' }}>{cmd.slice(0, cmdChars)}</span>
            </div>
            {[1, 2, 3].map((i) => (
              <CodeLine
                key={i}
                line={[{ text: `▌ [${i}/3] analyzing "skill-${i}"...`, color: 'violet' as const }]}
                start={progressStart + i * 6}
                cps={50}
              />
            ))}
            <div style={{ height: 6 }} />
            <CodeLine line={safe} start={safeStart} cps={50} />
            <CodeLine line={dangerous} start={dangerStart} cps={50} />
            <div style={{ height: 4 }} />
            <CodeLine
              line={[
                { text: 'Safe: ', color: 'green' as const },
                { text: '2', color: 'green' as const, bold: true },
                { text: '  │  ', color: 'muted' },
                { text: 'DANGER: ', color: 'red' as const },
                { text: '1', color: 'red' as const, bold: true },
              ]}
              start={totalStart}
              cps={50}
            />
            {showQuestion ? (
              <div
                style={{
                  fontFamily: fonts.code,
                  color: colors.text,
                  fontSize: 18,
                  display: 'flex',
                  gap: 8,
                  alignItems: 'center',
                }}
              >
                <span>
                  Disable skill <strong style={{ color: colors.red }}>"timebomb"</strong> due to
                  high risk? <span style={{ color: colors.amber }}>(Y/n)</span>
                </span>
                {cursorShown ? (
                  <span
                    style={{
                      display: 'inline-block',
                      width: 14,
                      height: 20,
                      background: blink ? colors.cyan : 'transparent',
                    }}
                  />
                ) : null}
              </div>
            ) : null}
            {showAnswer ? (
              <CodeLine
                line={[{ text: 'Y', color: 'green' as const, bold: true }]}
                start={answerStart}
                cps={50}
              />
            ) : null}
            {showSuccess ? (
              <>
                <CodeLine
                  line={[
                    { text: 'SUCCESS', color: 'green' as const, bold: true },
                    { text: ' Checked 3 skills, 1 high-risk, 1 disabled', color: 'green' as const },
                  ]}
                  start={successStart}
                  cps={50}
                />
                <CodeLine
                  line={[
                    { text: 'SUCCESS', color: 'green' as const, bold: true },
                    {
                      text: ' Synchronized project config: ~/.agents/.skill-organizer.yml',
                      color: 'green' as const,
                    },
                  ]}
                  start={successStart + 8}
                  cps={50}
                />
              </>
            ) : null}
          </div>
        }
      />
    </div>
  )
}
