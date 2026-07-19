import { ImageDithering, imageDitheringPresets } from '@paper-design/shaders-react'

const naturalPreset = imageDitheringPresets.find(p => p.name === 'Natural')!.params

export default function AsciiShader() {
  return (
    <ImageDithering
      {...naturalPreset}
      image="/hero.jpg"
      originalColors
      inverted={false}
      type="8x8"
      size={1.4}
      colorSteps={5}
      fit="cover"
      style={{ width: '100%', height: '100%' }}
    />
  )
}
