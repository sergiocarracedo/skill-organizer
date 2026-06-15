import { Img, staticFile } from 'remotion'
import { fonts } from '../lib/fonts'
import { colors } from '../lib/colors'

/**
 * Renders the real `logo_color2.png` banner (three colored
 * crystals pointing to a star) via Remotion's <Img>. Wordmark
 * is rendered as text next to the image. Use `wordmark=false`
 * for image-only (e.g. watermark / corner badges).
 */
export const BrandMark: React.FC<{ size?: number; wordmark?: boolean }> = ({
  size = 48,
  wordmark = false,
}) => {
  const height = size
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
      <Img
        src={staticFile('logo_color2.png')}
        style={{ height, width: 'auto', objectFit: 'contain' }}
      />
      {wordmark ? (
        <span
          style={{
            fontFamily: fonts.display,
            fontSize: size * 0.75,
            fontWeight: 600,
            color: colors.text,
            letterSpacing: '0.02em',
            textTransform: 'lowercase',
          }}
        >
          skill-organizer
        </span>
      ) : null}
    </div>
  )
}

export const BrandWord: React.FC<{ size?: number }> = ({ size = 36 }) => {
  return <BrandMark size={size} wordmark />
}
