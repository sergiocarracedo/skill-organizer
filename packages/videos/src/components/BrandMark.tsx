import { fonts } from '../lib/fonts'
import { colors } from '../lib/colors'

/**
 * React port of `packages/web/src/components/LogoMark.astro` —
 * the same three-blade triangular wedge used on the website
 * hero and `LogoMark.astro:1`.
 */
export const BrandMark: React.FC<{ size?: number }> = ({ size = 48 }) => {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 40 40"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-label="Skill Organizer logo"
    >
      <path
        d="M20 4 L36 36 L4 36 Z"
        fill={`url(#brand-grad)`}
        stroke={colors.cyan}
        strokeWidth={1.5}
        strokeLinejoin="round"
      />
      <path d="M20 14 L28 32 L12 32 Z" fill="rgba(0,0,0,0.5)" />
      <defs>
        <linearGradient
          id="brand-grad"
          x1="0"
          y1="0"
          x2="40"
          y2="40"
          gradientUnits="userSpaceOnUse"
        >
          <stop offset="0" stopColor={colors.cyan} stopOpacity={0.85} />
          <stop offset="0.5" stopColor={colors.violet} stopOpacity={0.65} />
          <stop offset="1" stopColor={colors.amber} stopOpacity={0.85} />
        </linearGradient>
      </defs>
    </svg>
  )
}

export const BrandWord: React.FC<{ size?: number }> = ({ size = 36 }) => {
  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 16,
        fontFamily: fonts.display,
        fontSize: size,
        fontWeight: 600,
        color: colors.text,
        letterSpacing: '0.02em',
        textTransform: 'lowercase',
      }}
    >
      <BrandMark size={size} />
      <span>skill-organizer</span>
    </div>
  )
}
