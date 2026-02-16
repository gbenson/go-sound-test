package main

import (
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

	const isCapture = false
	logDevices(isCapture)

	const deviceName = "" // "the most reasonable default"
	const sampleRate = 48000
	const sampleSizeBytes = 2 // 16-bit
	const numChannels = 1
	const bufferSizeFrames = 512 // (10 + 2/3)ms @ 48khz
	const bufferSizeBytes = 512 * numChannels * sampleSizeBytes

	want := sdl.AudioSpec{
		Freq:     int32(sampleRate),
		Format:   sdl.AUDIO_S16SYS, // signed 16-bit samples in native byte order
		Channels: 1,
		Samples:  uint16(bufferSizeBytes),
	}

	var spec sdl.AudioSpec
	deviceID, err := sdl.OpenAudioDevice(deviceName, isCapture, &want, &spec, 0)
	if err != nil {
		return err
	}
	defer sdl.CloseAudioDevice(deviceID)

	log.Info().
		Int("device_id", int(deviceID)).
		Int32("sample_rate", spec.Freq).
		Uint16("audio_format", uint16(spec.Format)).
		Uint8("num_channels", spec.Channels).
		Msg("Opened")

	var s16buf [bufferSizeFrames]int16
	for i := 0; i < bufferSizeFrames; i++ {
		s16buf[i] = int16(i*(1<<16)/bufferSizeFrames - (1 << 15))
		// if i&15 == 0 {
		// 	log.Printf("s16buf[%d] = %d", i, s16buf[i])
		// }
	}
	bbuf := (*[bufferSizeBytes]byte)(unsafe.Pointer(&s16buf))
	buf := bbuf[:]
	// for i := 0; i < bufferSizeBytes; i += 32 {
	// 	v := (int(buf[i+1]) << 8) | int(buf[i])
	// 	if v&0x8000 != 0 {
	// 		v = (v & 0x7fff) - 0x8000
	// 	}
	// 	log.Printf("buf[%d,%d] = %d (0x%x, 0x%x)", i, i+1, v, buf[i], buf[i+1])
	// }

	// Devices start paused
	sdl.PauseAudioDevice(deviceID, false)

	for {
		qas := sdl.GetQueuedAudioSize(deviceID)
		log.Debug().Uint32("queued_audio_size", qas).Msg("")
		if qas > 10000 {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		log.Debug().Int("buffer_bytes", len(buf)).Msg("Queueing")
		sdl.QueueAudio(deviceID, buf)
	}

	// go func() {
	// 	for true {
	// 		s.FillAudio()
	// 		time.Sleep(time.Millisecond * 17) // prevents fans from going crazy on Android handhelds
	// 	}
	// }()

	// return nil
	// for {
	// 	event := sdl.WaitEvent()
	// 	log.Printf("event: %T %v", event, event)
	// 	break
	// }

	return nil
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
