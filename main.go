package main

import (
	"fmt"
	"math/bits"
	"time"
	"unsafe"

	"gbenson.net/go/logger/log"
	"github.com/veandco/go-sdl2/sdl"
)

func main() {
	if err := _main(); err != nil {
		log.Err(err).Msg("")
	}
}

// Calculate the maximum number of samples we can queue without
// exceeding the given maximum latency at the given sample rate.
func calcMaxQueuedFrames(sampleRateHz int, maxLatency time.Duration) int {
	return int(time.Duration(sampleRateHz) * maxLatency / time.Second)
}

// // A Sink a
// type Sink interface {
// 	io.Closer
// 	SampleRate() int
// 	Input() chan<- float32
// }

// output := NewSDLSink

func _main() error {
	if err := sdl.Init(sdl.INIT_EVERYTHING); err != nil {
		return err
	}
	defer sdl.Quit()

	logDevices(false)

	sampleRate := 48000
	maxLatency := 10 * time.Millisecond

	maxQueuedFrames := calcMaxQueuedFrames(sampleRate, maxLatency)
	log.Info().
		Dur("for_max_latency", maxLatency).
		Int("need_max_queued_frames", maxQueuedFrames).
		Msg("")

	// https://wiki.libsdl.org/SDL2/SDL_OpenAudioDevice says:
	//   The desired size of the audio buffer, in _sample_frames_
	//   (e.g. with stereo output, two samples—left and right—would
	//   make a single sample frame).  Good values seem to range
	//   between 512 and 4096 inclusive, depending on the application
	//   and CPU speed.  Smaller values reduce latency but can lead
	//   to underflow if the application is doing heavy processing
	//   and cannot fill the audio buffer in time.
	bufferFrames := 1 << bits.Len(uint(maxQueuedFrames))
	log.Info().
		Int("buffer_frames", bufferFrames).
		Msg("Requesting")

	desiredSpec := sdl.AudioSpec{
		Freq:     int32(sampleRate),
		Format:   sdl.AUDIO_S16SYS, // signed 16-bit samples in native byte order
		Channels: 1,                // mono
		Samples:  uint16(bufferFrames),
	}

	var spec sdl.AudioSpec
	deviceID, err := sdl.OpenAudioDevice("", false, &desiredSpec, &spec, 0)
	if err != nil {
		return err
	}
	defer sdl.CloseAudioDevice(deviceID)

	log.Info().
		Int("device_id", int(deviceID)).
		Int32("sample_rate", spec.Freq).
		Uint16("audio_format", uint16(spec.Format)).
		Uint8("num_channels", spec.Channels).
		// audio buffer size in samples (power of 2)
		Uint16("buffer_size_samples", spec.Samples).
		// audio buffer size in bytes (calculated)
		Uint32("buffer_size_bytes", spec.Size).
		Msg("Opened")

	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel

	if spec.Format != sdl.AUDIO_S16SYS {
		return fmt.Errorf("unhandled sample format 0x%x", spec.Format)
	} else if spec.Channels != 1 {
		return fmt.Errorf("unhandled number of channels %d", spec.Channels)
		//} else if int(spec.Samples) != bufferFrames {
		//	return fmt.Errorf("unexpected buffer size %d", spec.Samples)
	}

	//maxQueuedFrames = calcMaxQueuedFrames(int(spec.Freq), maxLatency)
	//bufferFrames := maxQueuedFrames / 2 // fill half at a time so we never go over
	bufferBytes := bufferFrames * 2 // 16 bit mono => 2 bytes per sample

	// var wg sync.WaitGroup
	// c := make(chan int16, bufferFrames)

	buf := newMagicBuffer(bufferFrames)
	otherBuf := newMagicBuffer(bufferFrames)

	// Devices start paused...
	sdl.PauseAudioDevice(deviceID, false)

	// I'm requesting buffer_size = 512 frames
	// which is 1024 bytes of mono s16
	// or 2048 bytes of stereo s16
	// but the buffer depletes in 4096 byte chunks

	t := time.Now()
	count := 200 // 100 * 10ms = 1 second
	for count > 0 {
		qas := sdl.GetQueuedAudioSize(deviceID)
		if int(qas) > bufferBytes*4 {
			// if excessBytes > 0 {
			// 	excessTime := time.Duration(
			// 		int64(excessBytes) * int64(time.Second) / byteSampleRate,
			// 	)
			// 	log.Debug().
			// 		Int("size_bytes", queuedBytes).
			// 		Dur("excess_time", excessTime).
			// 		Msg("Queued audio")
			log.Info().
				Uint32("queued_audio_size", qas).
				Msg("Sleeping")
			time.Sleep(maxLatency / 10)
			continue
		}

		bufbuf := make([]byte, bufferBytes)
		copy(bufbuf, buf.Bytes)
		sdl.QueueAudio(deviceID, bufbuf)
		last := t
		t = time.Now()
		d := t.Sub(last)
		log.Info().
			Uint32("queued_audio_size", qas).
			Int("size_bytes", bufferBytes).
			Int64("delta_t", d.Microseconds()).
			Msg("Queueing")
		swap := buf
		buf = otherBuf
		otherBuf = swap

		// Generate test waveform
		frames := buf.Frames
		for i := range frames {
			frames[i] = int16((i<<16)/bufferFrames - (1 << 15))
		}
		// for i, v := range frames {
		// 	log.Info().Int("i", i).Int16("v", v).Msg("")
		// }

		// for i, want := range int16Buf {
		// 	lsb := int(bytesBuf[i*2])
		// 	msb := int(bytesBuf[i*2+1])
		// 	got := (msb << 8) | lsb
		// 	if got&0x8000 != 0 {
		// 		got = got&0x7fff - 0x8000
		// 	}
		// 	if got != int(want) {
		// 		panic("unexpected!")
		// 	}
		// }

		count--
	}

	return nil
}

type magicBuffer struct {
	Frames []int16
	Bytes  []byte
}

func newMagicBuffer(numFrames int) *magicBuffer {
	b := &magicBuffer{}
	b.Frames = make([]int16, numFrames)
	frameData := unsafe.SliceData(b.Frames)
	b.Bytes = unsafe.Slice((*byte)(unsafe.Pointer(frameData)), numFrames*2)
	return b
}

func logDevices(isCapture bool) {
	n := sdl.GetNumAudioDevices(isCapture)
	log.Debug().
		Bool("is_capture", isCapture).
		Int("num_devices", n).
		Msg("Found")

	for i := range n {
		name := sdl.GetAudioDeviceName(i, isCapture)
		spec, err := sdl.GetAudioDeviceSpec(i, isCapture)
		if err != nil {
			log.Err(err).
				Int("index", i).
				Str("name", name).
				Msg("Device")
			continue
		}
		log.Debug().
			Int("device_index", i).
			Str("name", name).
			Int32("sample_rate", spec.Freq).
			Uint16("audio_format", uint16(spec.Format)).
			Uint8("num_channels", spec.Channels).
			Msg("Found")
	}
}
