package main

// typedef unsigned char Uint8;
// void fillBuffer(void *userdata, Uint8 *stream, int len);
import "C"
import (
	"fmt"
	"math/bits"
	"runtime"
	"sync/atomic"
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

func _main() error {
	if err := sdl.Init(sdl.INIT_EVERYTHING); err != nil {
		return err
	}
	defer sdl.Quit()

	sink, err := NewSink(nil)
	if err != nil {
		return err
	}
	defer log.LoggedClose(sink, "sink")

	go func() {
		for i := 0; i < sink.sampleRate; i++ {
			v := (127 - (i & 256)) * 500
			sink.framesCh <- int16(v)
		}
		log.Info().Msg("Done sending")
	}()

	sink.Start()

	time.Sleep(2 * time.Second)
	log.Info().Msg("Done sleeping")

	return nil
}

const sinkMagic uint64 = 3141592653589793238

type Sink struct {
	magic      uint64
	sampleRate int
	framesCh   chan int16
	pinner     runtime.Pinner
	deviceID   sdl.AudioDeviceID
	isOpened   atomic.Bool
}

func NewSink(options *Options) (*Sink, error) {
	if options == nil {
		options = &Options{}
	}

	deviceName := options.DeviceName
	sampleRate := options.sampleRate()
	maxLatency := options.maxLatency()

	// Calculate the maximum number of frames we can buffer without
	// exceeding the required maximum latency at the requested sample
	// rate.
	maxBufferFrames := sampleRate * int(maxLatency) / int(time.Second)

	// SDL wants a power of two, so we round down from our maximum.
	// https://wiki.libsdl.org/SDL2/SDL_OpenAudioDevice says "good
	// values seem to range between 512 and 4096 inclusive, depending
	// on the application and CPU speed.  Smaller values reduce
	// latency but can lead to underflow if the application is doing
	// heavy processing and cannot fill the audio buffer in time."
	// Note that the sizes 512 and 4096 probably refer to _stereo_
	// audio: our 10ms default at 48kHz yields a 256 frame buffer.
	sdlBufferFrames := 1 << (bits.Len(uint(maxBufferFrames)) - 1)

	sink := &Sink{
		magic:      sinkMagic,
		sampleRate: sampleRate,
		framesCh:   make(chan int16, maxBufferFrames),
	}

	if err := sink.open(deviceName, sdlBufferFrames); err != nil {
		defer log.LoggedClose(sink, "sink")
		return nil, err
	}

	return sink, nil
}

func (sink *Sink) open(deviceName string, bufferFrames int) error {
	sink.pinner.Pin(sink)

	desiredSpec := sdl.AudioSpec{
		Freq:     int32(sink.sampleRate),
		Format:   sdl.AUDIO_S16SYS, // signed 16-bit samples in native byte order
		Channels: 1,                // mono
		Samples:  uint16(bufferFrames),
		Callback: sdl.AudioCallback(C.fillBuffer),
		UserData: unsafe.Pointer(sink),
	}

	var spec sdl.AudioSpec
	dev, err := sdl.OpenAudioDevice(deviceName, false, &desiredSpec, &spec, 0)
	if err != nil {
		return err
	}
	sink.deviceID = dev
	sink.isOpened.Store(true)

	log.Info().
		Int("device_id", int(sink.deviceID)).
		Int32("sample_rate", spec.Freq).
		Uint16("audio_format", uint16(spec.Format)).
		Uint8("num_channels", spec.Channels).
		Uint16("buffer_frames", spec.Samples).
		Uint32("buffer_bytes", spec.Size).
		Msg("Using SDL audio")

	if spec.Format != sdl.AUDIO_S16SYS {
		return fmt.Errorf("unexpected sample format 0x%x", spec.Format)
	} else if spec.Channels != 1 {
		return fmt.Errorf("unexpected number of channels (%d)", spec.Channels)
	}

	if int(spec.Freq) != sink.sampleRate {
		log.Warn().Int32("sample_rate", spec.Freq).Msg("Unexpected")
	}
	if int(spec.Samples) != bufferFrames {
		log.Warn().Uint16("buffer_size", spec.Samples).Msg("Unexpected")
	}

	return nil
}

// Close implements [io.Closer].
func (sink *Sink) Close() error {
	d := func(msg string) { log.Debug().Msg(msg) }

	defer sink.pinner.Unpin()
	defer d("Unpinning sink")

	if sink.isOpened.Swap(false) {
		defer sdl.CloseAudioDevice(sink.deviceID)
		defer d("Closing audio device")
	}

	close(sink.framesCh)
	defer d("Closing input channel")

	return nil
}

func (sink *Sink) Start() {
	sdl.PauseAudioDevice(sink.deviceID, false)
}

//export fillBuffer
func fillBuffer(receiver unsafe.Pointer, stream *C.Uint8, length C.int) {
	sink := (*Sink)(receiver)
	if got := sink.magic; got != sinkMagic {
		panic(fmt.Sprintf("invalid sink.magic %d (0x%x) != %d", got, got, sinkMagic))
	}

	src := sink.framesCh
	dst := unsafe.Slice((*int16)(unsafe.Pointer(stream)), int(length)/2)
	for i := range dst {
		dst[i] = <-src
	}
}
