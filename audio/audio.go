package audio

import (
	"encoding/binary"
	"math"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

// Sound effect identifiers.
const (
	SfxNone       = iota
	SfxAscending  // sector change, day/night transition ($60AA)
	SfxDescending // game over ($6B7F)
	SfxExplosion  // enemy bike destroyed ($6478)
	SfxCrash      // hit a tree ($6781)
	SfxBolt       // photon bolt fired
)

const sampleRate = 44100

// SoundStream implements io.Reader for oto, generating square waves.
type SoundStream struct {
	mu sync.Mutex

	// Engine hum (continuous)
	engineOn   bool
	engineFreq float64 // Hz
	enginePos  float64 // phase position in samples

	// Sound effect (interrupts engine briefly)
	sfxType     int
	sfxTime     float64 // elapsed time in seconds
	sfxDuration float64 // total duration in seconds
	sfxPos      float64 // phase position
}

// Read fills buf with float32LE samples. Called by oto.
func (s *SoundStream) Read(buf []byte) (int, error) {
	s.mu.Lock()

	engineOn := s.engineOn
	engineFreq := s.engineFreq
	enginePos := s.enginePos

	sfxType := s.sfxType
	sfxTime := s.sfxTime
	sfxDuration := s.sfxDuration
	sfxPos := s.sfxPos

	s.mu.Unlock()

	nSamples := len(buf) / 4 // 4 bytes per float32
	samplesWritten := 0

	for i := 0; i < nSamples; i++ {
		var sample float32

		if sfxType != SfxNone && sfxTime < sfxDuration {
			// Generate sound effect sample
			freq := sfxFrequency(sfxType, sfxTime, sfxDuration)
			vol := sfxVolume(sfxType, sfxTime, sfxDuration)

			// Square wave
			period := float64(sampleRate) / freq
			if sfxPos-math.Floor(sfxPos/period)*period < period/2 {
				sample = float32(vol)
			} else {
				sample = float32(-vol)
			}
			sfxPos++
			sfxTime += 1.0 / float64(sampleRate)
		} else if sfxType != SfxNone {
			// SFX finished
			sfxType = SfxNone
			sfxTime = 0
			sfxPos = 0
		}

		if sfxType == SfxNone && engineOn {
			// Engine hum: low frequency square wave
			period := float64(sampleRate) / engineFreq
			if enginePos-math.Floor(enginePos/period)*period < period/2 {
				sample = 0.08
			} else {
				sample = -0.08
			}
			enginePos++
		}

		off := i * 4
		binary.LittleEndian.PutUint32(buf[off:off+4], math.Float32bits(sample))
		samplesWritten++
	}

	// Write back updated state
	s.mu.Lock()
	s.enginePos = enginePos
	if sfxType == SfxNone && s.sfxType != SfxNone {
		// SFX finished during this Read
		s.sfxType = SfxNone
		s.sfxTime = 0
		s.sfxPos = 0
	} else {
		s.sfxTime = sfxTime
		s.sfxPos = sfxPos
	}
	s.mu.Unlock()

	return samplesWritten * 4, nil
}

// sfxFrequency returns the instantaneous frequency for a sound effect.
func sfxFrequency(typ int, t, dur float64) float64 {
	progress := t / dur

	switch typ {
	case SfxAscending:
		// $60AA: E starts at 20, decreases → freq rises.
		// Sweep from ~200Hz to ~2000Hz over duration.
		return 200 + 1800*progress

	case SfxDescending:
		// $6B7F: E starts at 30, increases → freq drops.
		// Sweep from ~1500Hz down to ~100Hz.
		return 1500 - 1400*progress

	case SfxExplosion:
		// $6478: short buzzy explosion — noise-like rapid frequency changes.
		// Use a low frequency that wobbles.
		base := 120.0 - 80.0*progress
		// Add pseudo-noise by using sin of time to modulate
		wobble := 50.0 * math.Sin(t*800)
		return math.Max(40, base+wobble)

	case SfxCrash:
		// $6781: descending buzz, short.
		return 800 - 600*progress

	case SfxBolt:
		// Short high-pitched zap
		return 3000 - 2000*progress
	}

	return 440
}

// sfxVolume returns the volume envelope for a sound effect.
func sfxVolume(typ int, t, dur float64) float64 {
	progress := t / dur

	switch typ {
	case SfxExplosion:
		// Loud start, quick decay
		return 0.25 * (1.0 - progress*0.7)
	case SfxCrash:
		return 0.2 * (1.0 - progress*0.5)
	case SfxBolt:
		// Very short, moderate volume
		return 0.15 * (1.0 - progress)
	case SfxAscending, SfxDescending:
		// Steady volume with slight fadeout
		return 0.18 * (1.0 - progress*0.3)
	}
	return 0.15
}

// Audio manages the sound output.
type Audio struct {
	stream *SoundStream
	player *oto.Player
}

// New creates and starts the audio system.
func New() *Audio {
	stream := &SoundStream{
		engineFreq: 55, // low A — characteristic engine drone
	}

	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   sampleRate,
		ChannelCount: 1,
		Format:       oto.FormatFloat32LE,
	})
	if err != nil {
		// Audio is optional — return a silent stub if it fails.
		return &Audio{stream: stream}
	}
	<-ready

	player := ctx.NewPlayer(stream)
	player.SetBufferSize(4096)
	player.Play()

	return &Audio{
		stream: stream,
		player: player,
	}
}

// StartEngine enables the continuous engine hum.
func (a *Audio) StartEngine() {
	a.stream.mu.Lock()
	a.stream.engineOn = true
	a.stream.mu.Unlock()
}

// StopEngine disables the engine hum.
func (a *Audio) StopEngine() {
	a.stream.mu.Lock()
	a.stream.engineOn = false
	a.stream.mu.Unlock()
}

// Play triggers a sound effect, interrupting any current SFX.
func (a *Audio) Play(sfx int) {
	var dur time.Duration
	switch sfx {
	case SfxAscending:
		dur = 600 * time.Millisecond
	case SfxDescending:
		dur = 800 * time.Millisecond
	case SfxExplosion:
		dur = 300 * time.Millisecond
	case SfxCrash:
		dur = 250 * time.Millisecond
	case SfxBolt:
		dur = 100 * time.Millisecond
	default:
		return
	}

	a.stream.mu.Lock()
	a.stream.sfxType = sfx
	a.stream.sfxTime = 0
	a.stream.sfxDuration = dur.Seconds()
	a.stream.sfxPos = 0
	a.stream.mu.Unlock()
}
