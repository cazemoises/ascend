const MUTE_KEY = 'ascend:sound-muted'

export function isSoundMuted(): boolean {
  try {
    return localStorage.getItem(MUTE_KEY) === 'true'
  } catch {
    return false
  }
}

export function setSoundMuted(muted: boolean): void {
  try {
    localStorage.setItem(MUTE_KEY, String(muted))
  } catch {
    // localStorage unavailable (private mode, etc.) — the toggle still
    // works for the current session via component state.
  }
}

// C5 -> E5 -> G5: a quick ascending three-note chime.
const NOTES_HZ = [523.25, 659.25, 783.99]
const NOTE_DURATION_S = 0.14
const PEAK_GAIN = 0.12

let audioCtx: AudioContext | null = null

function getAudioContext(): AudioContext {
  if (!audioCtx) {
    audioCtx = new AudioContext()
  }
  return audioCtx
}

export function playAcceptedChime(): void {
  if (isSoundMuted()) return

  const ctx = getAudioContext()
  const now = ctx.currentTime

  NOTES_HZ.forEach((freq, i) => {
    const start = now + i * NOTE_DURATION_S
    const osc = ctx.createOscillator()
    const gain = ctx.createGain()

    osc.type = 'sine'
    osc.frequency.value = freq

    // Short attack/release envelope so each note is a soft "ping" instead
    // of a click, and the whole chime stays at a moderate volume.
    gain.gain.setValueAtTime(0, start)
    gain.gain.linearRampToValueAtTime(PEAK_GAIN, start + 0.02)
    gain.gain.linearRampToValueAtTime(0, start + NOTE_DURATION_S)

    osc.connect(gain)
    gain.connect(ctx.destination)

    osc.start(start)
    osc.stop(start + NOTE_DURATION_S)
  })
}
