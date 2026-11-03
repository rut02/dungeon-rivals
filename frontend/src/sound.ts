// Retro 8-bit Sound Synthesizer via Web Audio API (Zero external assets)

class SoundFX {
  private ctx: AudioContext | null = null

  private getContext(): AudioContext | null {
    if (typeof window === 'undefined') return null
    if (!this.ctx) {
      const AudioCtx = window.AudioContext || (window as any).webkitAudioContext
      if (AudioCtx) {
        this.ctx = new AudioCtx()
      }
    }
    if (this.ctx && this.ctx.state === 'suspended') {
      this.ctx.resume()
    }
    return this.ctx
  }

  playSlash() {
    const ctx = this.getContext()
    if (!ctx) return
    const now = ctx.currentTime
    const osc = ctx.createOscillator()
    const gain = ctx.createGain()

    osc.type = 'sawtooth'
    osc.frequency.setValueAtTime(450, now)
    osc.frequency.exponentialRampToValueAtTime(80, now + 0.12)

    gain.gain.setValueAtTime(0.25, now)
    gain.gain.exponentialRampToValueAtTime(0.01, now + 0.12)

    osc.connect(gain)
    gain.connect(ctx.destination)

    osc.start(now)
    osc.stop(now + 0.13)
  }

  playArrow() {
    const ctx = this.getContext()
    if (!ctx) return
    const now = ctx.currentTime
    const osc = ctx.createOscillator()
    const gain = ctx.createGain()

    osc.type = 'triangle'
    osc.frequency.setValueAtTime(900, now)
    osc.frequency.exponentialRampToValueAtTime(220, now + 0.1)

    gain.gain.setValueAtTime(0.2, now)
    gain.gain.exponentialRampToValueAtTime(0.01, now + 0.1)

    osc.connect(gain)
    gain.connect(ctx.destination)

    osc.start(now)
    osc.stop(now + 0.11)
  }

  playMagic() {
    const ctx = this.getContext()
    if (!ctx) return
    const now = ctx.currentTime
    const osc = ctx.createOscillator()
    const gain = ctx.createGain()

    osc.type = 'sine'
    osc.frequency.setValueAtTime(300, now)
    osc.frequency.linearRampToValueAtTime(750, now + 0.1)
    osc.frequency.exponentialRampToValueAtTime(100, now + 0.28)

    gain.gain.setValueAtTime(0.25, now)
    gain.gain.exponentialRampToValueAtTime(0.01, now + 0.28)

    osc.connect(gain)
    gain.connect(ctx.destination)

    osc.start(now)
    osc.stop(now + 0.29)
  }

  playShield() {
    const ctx = this.getContext()
    if (!ctx) return
    const now = ctx.currentTime
    const osc = ctx.createOscillator()
    const gain = ctx.createGain()

    osc.type = 'square'
    osc.frequency.setValueAtTime(220, now)
    osc.frequency.setValueAtTime(440, now + 0.04)
    osc.frequency.exponentialRampToValueAtTime(110, now + 0.18)

    gain.gain.setValueAtTime(0.2, now)
    gain.gain.exponentialRampToValueAtTime(0.01, now + 0.18)

    osc.connect(gain)
    gain.connect(ctx.destination)

    osc.start(now)
    osc.stop(now + 0.19)
  }

  playCrit() {
    const ctx = this.getContext()
    if (!ctx) return
    const now = ctx.currentTime
    const osc1 = ctx.createOscillator()
    const osc2 = ctx.createOscillator()
    const gain = ctx.createGain()

    osc1.type = 'sawtooth'
    osc2.type = 'sine'
    osc1.frequency.setValueAtTime(800, now)
    osc1.frequency.exponentialRampToValueAtTime(150, now + 0.2)
    osc2.frequency.setValueAtTime(1200, now)
    osc2.frequency.exponentialRampToValueAtTime(300, now + 0.2)

    gain.gain.setValueAtTime(0.3, now)
    gain.gain.exponentialRampToValueAtTime(0.01, now + 0.2)

    osc1.connect(gain)
    osc2.connect(gain)
    gain.connect(ctx.destination)

    osc1.start(now)
    osc2.start(now)
    osc1.stop(now + 0.21)
    osc2.stop(now + 0.21)
  }

  playLevelUp() {
    const ctx = this.getContext()
    if (!ctx) return
    const now = ctx.currentTime
    const notes = [330, 392, 494, 659] // E4, G4, B4, E5
    notes.forEach((freq, i) => {
      const osc = ctx.createOscillator()
      const gain = ctx.createGain()
      const startTime = now + i * 0.08
      osc.type = 'triangle'
      osc.frequency.setValueAtTime(freq, startTime)
      gain.gain.setValueAtTime(0.2, startTime)
      gain.gain.exponentialRampToValueAtTime(0.01, startTime + 0.18)
      osc.connect(gain)
      gain.connect(ctx.destination)
      osc.start(startTime)
      osc.stop(startTime + 0.19)
    })
  }

  playDefeat() {
    const ctx = this.getContext()
    if (!ctx) return
    const now = ctx.currentTime
    const notes = [400, 350, 300, 220]
    notes.forEach((freq, i) => {
      const osc = ctx.createOscillator()
      const gain = ctx.createGain()
      const startTime = now + i * 0.12
      osc.type = 'sawtooth'
      osc.frequency.setValueAtTime(freq, startTime)
      gain.gain.setValueAtTime(0.2, startTime)
      gain.gain.exponentialRampToValueAtTime(0.01, startTime + 0.15)
      osc.connect(gain)
      gain.connect(ctx.destination)
      osc.start(startTime)
      osc.stop(startTime + 0.16)
    })
  }
}

export const sound = new SoundFX()
