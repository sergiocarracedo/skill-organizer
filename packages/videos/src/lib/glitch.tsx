import { useCurrentFrame } from 'remotion'
import { colors } from './colors'

/**
 * Seeded mulberry32 PRNG. Returns a function that yields a new
 * pseudo-random float in [0, 1) per call. Used so the glitch
 * pattern is deterministic across re-renders — the same frame
 * number always produces the same visual offset.
 */
export function makeRng(seed: number) {
  let a = seed >>> 0
  return () => {
    a = (a + 0x6d2b79f5) >>> 0
    let t = a
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

export type GlitchOffset = {
  x: number
  y: number
  redShiftX: number
  cyanShiftX: number
  scanlineOpacity: number
  sliceShiftPx: number
  sliceOffset: number
  shakeX: number
  shakeY: number
  hue: number
}

/**
 * Map `intensity` in [0, 1] to the full offset struct.
 * `frame` shifts the seed so the pattern evolves over time.
 * Per Remotion best-practice: every channel is `interpolate()`
 * or seeded — no CSS animations.
 */
export function glitchOffset(frame: number, intensity: number): GlitchOffset {
  const rng = makeRng(frame * 9301 + 49297)
  const baseShift = intensity * 6
  const peak = (rng() * 2 - 1) * baseShift
  const peakY = (rng() * 2 - 1) * baseShift * 0.4
  return {
    x: peak,
    y: peakY,
    redShiftX: peak + intensity * 4,
    cyanShiftX: -peak - intensity * 4,
    scanlineOpacity: 0.12 + intensity * 0.5,
    sliceShiftPx: peak * 1.5,
    sliceOffset: Math.floor(rng() * 12),
    shakeX: peak * 0.4,
    shakeY: peakY * 0.4,
    hue: rng() * 30 - 15,
  }
}

/**
 * Convenience hook. Equivalent to `glitchOffset(useCurrentFrame(), i)`.
 */
export function useGlitchOffset(intensity: number): GlitchOffset {
  const frame = useCurrentFrame()
  return glitchOffset(frame, intensity)
}

export { colors }
