export const colors = {
  bg: '#020611',
  panel: 'rgba(9, 16, 33, 0.74)',
  panelStrong: 'rgba(14, 22, 42, 0.94)',
  border: 'rgba(154, 188, 255, 0.16)',
  text: '#edf5ff',
  muted: 'rgba(237, 245, 255, 0.72)',
  cyan: '#43eeff',
  violet: '#9d61ff',
  amber: '#ffb549',
  red: '#ff5a5a',
  green: '#7aef9a',
  rose: '#ff6a8a',
} as const

export type ColorKey = keyof typeof colors
