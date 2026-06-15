import { Composition, registerRoot } from 'remotion'
import { SecurityCheckPromo } from './SecurityCheckPromo'

export const RemotionRoot: React.FC = () => {
  return (
    <Composition
      id="security-check-promo"
      component={SecurityCheckPromo}
      durationInFrames={750}
      fps={30}
      width={1920}
      height={1080}
    />
  )
}

registerRoot(RemotionRoot)
