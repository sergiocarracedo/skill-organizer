import { interpolate, useCurrentFrame } from 'remotion'
import { fonts } from '../lib/fonts'
import { colors } from '../lib/colors'

export type CodeChunk = { text: string; color?: string; bold?: boolean }
export type CodeLine = string | CodeChunk[]

const colorMap: Record<string, string> = {
  default: 'rgba(235, 244, 255, 0.94)',
  white: 'rgba(235, 244, 255, 0.94)',
  muted: 'rgba(197, 211, 229, 0.66)',
  cyan: colors.cyan,
  violet: 'rgba(196, 166, 255, 0.92)',
  amber: 'rgba(255, 191, 98, 0.92)',
  green: 'rgba(112, 246, 177, 0.94)',
  red: 'rgba(255, 130, 130, 0.92)',
}

/**
 * Type-on code line. `start` is the frame the line begins, `cps`
 * is characters-per-second. When `start` is in the past the full
 * text is shown; the line eases in char-by-char.
 */
export const CodeLine: React.FC<{
  line: CodeLine
  start: number
  cps?: number
  size?: number
}> = ({ line, start, cps = 40, size = 18 }) => {
  const frame = useCurrentFrame()
  const elapsed = (frame - start) / 30
  const visible = Math.max(0, Math.floor(elapsed * cps))
  const chunks: CodeChunk[] = typeof line === 'string' ? [{ text: line }] : line
  const flat = chunks.map((c) => c.text).join('')
  const shown = flat.slice(0, visible)
  const y = interpolate(frame, [start, start + 6], [4, 0], { extrapolateRight: 'clamp' })
  const opacity = interpolate(frame, [start, start + 8], [0, 1], { extrapolateRight: 'clamp' })
  return (
    <div
      style={{
        fontFamily: fonts.code,
        fontSize: size,
        lineHeight: 1.55,
        opacity: frame < start ? 0 : opacity,
        transform: `translateY(${y}px)`,
        whiteSpace: 'pre',
      }}
    >
      {chunks.map((chunk, i) => {
        const before = chunks.slice(0, i).reduce((n, c) => n + c.text.length, 0)
        const after = before + chunk.text.length
        const text = shown.slice(before, after)
        if (!text) return null
        return (
          <span
            key={i}
            style={{
              color: colorMap[chunk.color ?? 'default'],
              fontWeight: chunk.bold ? 700 : 400,
            }}
          >
            {text}
          </span>
        )
      })}
    </div>
  )
}
