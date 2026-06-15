import { loadFont } from '@remotion/google-fonts/Oxanium'
import { loadFont as loadMono } from '@remotion/google-fonts/JetBrainsMono'

const ox = loadFont('normal')
const jb = loadMono('normal')

export const fonts = {
  display: ox.fontFamily,
  code: jb.fontFamily,
  displayReady: ox.waitUntilDone,
  codeReady: jb.waitUntilDone,
}
