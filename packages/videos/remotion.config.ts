import { Config } from '@remotion/cli/config'

Config.setVideoImageFormat('jpeg')
Config.setConcurrency(1)
Config.setChromiumOpenGlRenderer('angle')
Config.setChromiumHeadlessMode(true)
Config.setOverwriteOutput(true)
