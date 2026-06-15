import { describe, expect, it } from 'vitest'
import { glitchOffset, makeRng } from '../src/lib/glitch'

describe('makeRng', () => {
  it('returns floats in [0, 1)', () => {
    const rng = makeRng(42)
    for (let i = 0; i < 1000; i++) {
      const r = rng()
      expect(r).toBeGreaterThanOrEqual(0)
      expect(r).toBeLessThan(1)
    }
  })

  it('is deterministic for the same seed', () => {
    const a = makeRng(123)
    const b = makeRng(123)
    for (let i = 0; i < 50; i++) {
      expect(a()).toBe(b())
    }
  })

  it('produces different streams for different seeds', () => {
    const a = makeRng(1)
    const b = makeRng(2)
    let same = 0
    for (let i = 0; i < 20; i++) {
      if (a() === b()) same++
    }
    expect(same).toBe(0)
  })
})

describe('glitchOffset', () => {
  it('scales the chromatic split with intensity', () => {
    const low = glitchOffset(0, 0)
    const high = glitchOffset(0, 1)
    expect(high.redShiftX).toBeGreaterThan(Math.abs(low.redShiftX))
  })

  it('returns the same offset for the same frame + intensity', () => {
    expect(glitchOffset(42, 0.5)).toEqual(glitchOffset(42, 0.5))
  })

  it('returns a zero-ish shift at intensity 0', () => {
    const o = glitchOffset(123, 0)
    expect(o.redShiftX).toBeCloseTo(0)
    expect(o.cyanShiftX).toBeCloseTo(0)
    expect(o.scanlineOpacity).toBeCloseTo(0.12)
  })
})
