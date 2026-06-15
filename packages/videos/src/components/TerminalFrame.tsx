import { fonts } from '../lib/fonts'
import { colors } from '../lib/colors'

type Props = {
  title?: string
  body: React.ReactNode
  width?: number | string
  height?: number | string
  className?: string
  style?: React.CSSProperties
}

/**
 * Mirrors `packages/web/src/components/TerminalFrame.astro` —
 * traffic-light dots, title bar, monospace body, dark panel.
 * No Astro dependency, no CSS animations: all positioning is
 * done with inline styles.
 */
export const TerminalFrame: React.FC<Props> = ({
  title,
  body,
  width,
  height,
  className,
  style,
}) => {
  return (
    <div
      className={className}
      style={{
        width: width ?? '100%',
        height: height ?? '100%',
        overflow: 'hidden',
        borderRadius: 24,
        border: `1px solid ${colors.border}`,
        background: 'rgba(7, 11, 21, 0.94)',
        boxShadow: '0 32px 90px rgba(7, 13, 26, 0.48)',
        fontFamily: fonts.code,
        color: colors.text,
        display: 'flex',
        flexDirection: 'column',
        ...style,
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 9,
          borderBottom: `1px solid ${colors.border}`,
          background: 'rgba(255,255,255,0.03)',
          padding: '14px 18px',
        }}
      >
        <span style={dotStyle('rgba(255, 109, 65, 0.84)')} />
        <span style={dotStyle('rgba(255, 181, 73, 0.84)')} />
        <span style={dotStyle('rgba(67, 238, 255, 0.84)')} />
        {title ? (
          <h6
            style={{
              fontFamily: fonts.code,
              fontSize: 14,
              color: 'rgba(235, 244, 255, 0.82)',
              margin: 0,
              marginLeft: 12,
              fontWeight: 400,
            }}
          >
            {title}
          </h6>
        ) : null}
      </div>
      <div
        style={{
          padding: '20px 22px',
          fontSize: 18,
          lineHeight: 1.55,
          flex: 1,
          overflow: 'hidden',
        }}
      >
        {body}
      </div>
    </div>
  )
}

const dotStyle = (bg: string): React.CSSProperties => ({
  display: 'inline-block',
  width: 12,
  height: 12,
  borderRadius: '50%',
  background: bg,
  flexShrink: 0,
})
