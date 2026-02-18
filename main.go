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

const (
	DefaultSampleRate = 48000
	DefaultMaxLatency = 10 * time.Millisecond
)

type Options struct {
	DeviceName string
	SampleRate int
	MaxLatency time.Duration
}

func (o *Options) sampleRate() int {
	if o.SampleRate > 0 {
		return o.SampleRate
	}
	return DefaultSampleRate
}

func (o *Options) maxLatency() time.Duration {
	if o.MaxLatency > 0 {
		return o.MaxLatency
	}
	return DefaultMaxLatency
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

	sink.Start()
	time.Sleep(3 * time.Second)

	return nil
}

type Sink struct {
	pinner     runtime.Pinner
	deviceID   sdl.AudioDeviceID
	isOpen     atomic.Bool
	deviceSpec sdl.AudioSpec
}

func NewSink(options *Options) (sink *Sink, err error) {
	if options == nil {
		options = &Options{}
	}

	sink = &Sink{}
	if err := sink.open(options); err != nil {
		defer log.LoggedClose(sink, "sink")
	}

	return
}

func (sink *Sink) open(options *Options) error {
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
	bufferFrames := 1 << (bits.Len(uint(maxBufferFrames)) - 1)

	sinkPtr := unsafe.Pointer(sink)
	sink.pinner.Pin(sinkPtr)

	desiredSpec := sdl.AudioSpec{
		Freq:     int32(sampleRate),
		Format:   sdl.AUDIO_S16SYS, // signed 16-bit samples in native byte order
		Channels: 1,                // mono
		Samples:  uint16(bufferFrames),
		Callback: sdl.AudioCallback(C.fillBuffer),
		UserData: sinkPtr,
	}
	sink.pinner.Pin(&desiredSpec)
	sink.pinner.Pin(desiredSpec.Callback)
	sink.pinner.Pin(desiredSpec.UserData)

	spec := &sink.deviceSpec
	dev, err := sdl.OpenAudioDevice(deviceName, false, &desiredSpec, spec, 0)
	if err != nil {
		return err
	}
	sink.deviceID = dev
	sink.isOpen.Store(true)

	log.Info().
		Int("device_id", int(sink.deviceID)).
		Int32("sample_rate", spec.Freq).
		Uint16("audio_format", uint16(spec.Format)).
		Uint8("num_channels", spec.Channels).
		Uint16("buffer_num_frames", spec.Samples).
		Uint32("buffer_size_bytes", spec.Size).
		Msg("Sink")

	if spec.Format != sdl.AUDIO_S16SYS {
		return fmt.Errorf("unexpected sample format 0x%x", spec.Format)
	} else if spec.Channels != 1 {
		return fmt.Errorf("unexpected number of channels (%d)", spec.Channels)
	} else if int(spec.Samples) != bufferFrames {
		return fmt.Errorf("unexpected buffer size (%d frames)", spec.Samples)
	}

	return nil
}

// Close implements [io.Closer].
func (sink *Sink) Close() error {
	defer sink.pinner.Unpin()

	if sink.isOpen.Swap(false) {
		defer sdl.CloseAudioDevice(sink.deviceID)
	}

	return nil
}

func (sink *Sink) Start() {
	sdl.PauseAudioDevice(sink.deviceID, false)
}

//export fillBuffer
func fillBuffer(userdata unsafe.Pointer, stream *C.Uint8, length C.int) {
	fmt.Printf("ptr=%v stream=%v len=%v\n", userdata, stream, length)
}
