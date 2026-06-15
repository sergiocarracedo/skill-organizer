import { AbsoluteFill } from 'remotion'
import { useGlitchOffset } from '../lib/glitch'

type Props = {
  intensity: number
  children: React.ReactNode
  variant?: 'rgb-split' | 'slice' | 'full'
}

/**
 * Layered glitch overlay. Applies (a) RGB split, (b) horizontal
 * slice shifts, (c) scanlines, and (d) a small camera shake.
 * All offsets are derived from `glitchOffset()` which is itself
 * a function of `useCurrentFrame()` — no CSS animations.
 */
export const GlitchOverlay: React.FC<Props> = ({ intensity, children, variant = 'full' }) => {
  const o = useGlitchOffset(intensity)
  const showRgb = variant !== 'slice'
  const showSlice = variant !== 'rgb-split'
  const showScan = variant === 'full'
  return (
    <AbsoluteFill
      style={{
        transform: `translate(${o.shakeX}px, ${o.shakeY}px)`,
      }}
    >
      <AbsoluteFill
        style={{
          transform: `translate(${o.x}px, ${o.y}px)`,
          filter: `hue-rotate(${o.hue}deg)`,
        }}
      >
        {children}
      </AbsoluteFill>
      {showRgb && intensity > 0.05 ? (
        <>
          <AbsoluteFill
            style={{
              mixBlendMode: 'screen',
              opacity: Math.min(0.6, intensity * 0.7),
              transform: `translate(${o.redShiftX}px, 0)`,
              filter: 'url(#red-channel)',
            }}
          >
            <div
              style={{
                width: '100%',
                height: '100%',
                background: 'rgba(255, 32, 32, 0.18)',
                mixBlendMode: 'multiply',
              }}
            />
          </AbsoluteFill>
          <AbsoluteFill
            style={{
              mixBlendMode: 'screen',
              opacity: Math.min(0.6, intensity * 0.7),
              transform: `translate(${o.cyanShiftX}px, 0)`,
            }}
          >
            <div
              style={{
                width: '100%',
                height: '100%',
                background: 'rgba(32, 200, 255, 0.16)',
                mixBlendMode: 'multiply',
              }}
            />
          </AbsoluteFill>
        </>
      ) : null}
      {showSlice && intensity > 0.1 ? (
        <>
          {[0, 1, 2].map((i) => (
            <div
              key={i}
              style={{
                position: 'absolute',
                left: 0,
                right: 0,
                top: `${20 + i * 26 + o.sliceOffset * 0.6}%`,
                height: 22,
                background: 'rgba(255, 90, 90, 0.18)',
                transform: `translateX(${o.sliceShiftPx * (i + 1) * 0.6}px)`,
                mixBlendMode: 'screen',
                opacity: 0.4 + intensity * 0.5,
              }}
            />
          ))}
        </>
      ) : null}
      {showScan ? (
        <AbsoluteFill
          style={{
            backgroundImage: `repeating-linear-gradient(
              to bottom,
              rgba(0, 0, 0, 0) 0,
              rgba(0, 0, 0, 0) 3px,
              rgba(0, 0, 0, ${0.32 + intensity * 0.3}) 3px,
              rgba(0, 0, 0, ${0.32 + intensity * 0.3}) 4px
            )`,
            opacity: o.scanlineOpacity,
            mixBlendMode: 'multiply',
            pointerEvents: 'none',
          }}
        />
      ) : null}
      {intensity > 0.6 ? (
        <AbsoluteFill
          style={{
            background: `linear-gradient(0deg, rgba(255,40,40,${(intensity - 0.6) * 0.4}) 0%, transparent 50%, rgba(255,40,40,${
              (intensity - 0.6) * 0.5
            }) 100%)`,
            mixBlendMode: 'screen',
          }}
        />
      ) : null}
    </AbsoluteFill>
  )
}
