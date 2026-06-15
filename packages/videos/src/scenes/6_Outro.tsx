import { interpolate, useCurrentFrame } from 'remotion'
import { BrandWord } from '../components/BrandMark'
import { fonts } from '../lib/fonts'
import { colors } from '../lib/colors'

type Props = { baseStart: number }

export const Scene6_Outro: React.FC<Props> = ({ baseStart }) => {
  const frame = useCurrentFrame()
  const local = frame - baseStart
  const fadeIn = interpolate(local, [0, 20], [0, 1], { extrapolateRight: 'clamp' })
  const slideUp = interpolate(local, [0, 24], [40, 0], { extrapolateRight: 'clamp' })
  const urlShown = local > 40

  return (
    <div
      style={{
        position: 'absolute',
        inset: 0,
        background: '#020611',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 32,
        opacity: fadeIn,
        transform: `translateY(${slideUp}px)`,
      }}
    >
      <div
        style={{
          fontFamily: fonts.display,
          fontSize: 72,
          fontWeight: 600,
          color: colors.text,
          letterSpacing: '-0.02em',
          textAlign: 'center',
          lineHeight: 1.1,
        }}
      >
        Score every skill.
      </div>
      <div
        style={{
          fontFamily: fonts.display,
          fontSize: 36,
          color: 'rgba(67, 238, 255, 0.85)',
          letterSpacing: '0.02em',
        }}
      >
        before it runs.
      </div>
      <div
        style={{
          marginTop: 48,
          opacity: urlShown
            ? interpolate(local - 40, [0, 12], [0, 1], { extrapolateRight: 'clamp' })
            : 0,
          transform: urlShown
            ? `translateY(${interpolate(local - 40, [0, 12], [10, 0], { extrapolateRight: 'clamp' })}px)`
            : 'translateY(10px)',
        }}
      >
        <BrandWord size={36} />
      </div>
      <div
        style={{
          fontFamily: fonts.code,
          fontSize: 22,
          color: 'rgba(197, 211, 229, 0.7)',
          marginTop: 16,
        }}
      >
        $ npm i -g skill-organizer
      </div>
    </div>
  )
}
