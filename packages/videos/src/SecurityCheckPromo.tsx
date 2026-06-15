import { AbsoluteFill, Sequence } from 'remotion'
import { Scene1_OpencodeLoads } from './scenes/1_OpencodeLoads'
import { Scene2_MaliciousCode } from './scenes/2_MaliciousCode'
import { Scene3_SystemDegrades } from './scenes/3_SystemDegrades'
import { Scene4_NewTerminal } from './scenes/4_NewTerminal'
import { Scene5_CheckSecurity } from './scenes/5_CheckSecurity'
import { Scene6_Outro } from './scenes/6_Outro'

/**
 * 1920×1080 @ 30 fps, 750 frames (25 s). 6 scenes, each is a
 * `<Sequence>` with absolute fill and `layout="none"` so the
 * scene receives `useCurrentFrame()` values from 0 to
 * `durationInFrames` (not absolute).
 */
const F = 30
const S = (s: number) => s * F
const FRAMES = {
  intro: S(0),
  malicious: S(4),
  degrades: S(9),
  rescue: S(13),
  check: S(16),
  outro: S(22),
}
const DUR = {
  intro: S(4),
  malicious: S(5),
  degrades: S(4),
  rescue: S(3),
  check: S(6),
  outro: S(3),
}

export const SecurityCheckPromo: React.FC = () => {
  return (
    <AbsoluteFill style={{ background: '#020611' }}>
      <Sequence from={FRAMES.intro} durationInFrames={DUR.intro} layout="none">
        <Scene1_OpencodeLoads baseStart={0} />
      </Sequence>
      <Sequence from={FRAMES.malicious} durationInFrames={DUR.malicious} layout="none">
        <Scene2_MaliciousCode baseStart={0} />
      </Sequence>
      <Sequence from={FRAMES.degrades} durationInFrames={DUR.degrades} layout="none">
        <Scene3_SystemDegrades baseStart={0} />
      </Sequence>
      <Sequence from={FRAMES.rescue} durationInFrames={DUR.rescue} layout="none">
        <Scene4_NewTerminal baseStart={0} />
      </Sequence>
      <Sequence from={FRAMES.check} durationInFrames={DUR.check} layout="none">
        <Scene5_CheckSecurity baseStart={0} />
      </Sequence>
      <Sequence from={FRAMES.outro} durationInFrames={DUR.outro} layout="none">
        <Scene6_Outro baseStart={0} />
      </Sequence>
    </AbsoluteFill>
  )
}
