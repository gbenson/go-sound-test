package main

import (
	"log"

	"github.com/veandco/go-sdl2/sdl"
)

func main() {
	if err := _main(); err != nil {
		log.Fatalln("error:", err)
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

	desiredSpec := sdl.AudioSpec{
		Freq:     int32(sampleRate),
		Format:   sdl.AUDIO_S16SYS, // signed 16-bit samples in native byte order
		Channels: 1,
		Samples:  uint16(bufferSizeBytes),
	}

	var spec sdl.AudioSpec
	deviceID, err := sdl.OpenAudioDevice(deviceName, isCapture, &desiredSpec, &spec, 0)
	if err != nil {
		return err
	}
	defer sdl.CloseAudioDevice(deviceID)

	log.Printf("opened device id=%d spec=%v", deviceID, spec)

	return nil
}

func logDevices(isCapture bool) {
	n := sdl.GetNumAudioDevices(isCapture)
	log.Printf("devices: is_capture=%v count=%d", isCapture, n)

	for i := range n {
		name := sdl.GetAudioDeviceName(i, isCapture)
		spec, err := sdl.GetAudioDeviceSpec(i, isCapture)
		if err != nil {
			log.Printf("device index=%d name=%q error=%s", i, name, err.Error())
			continue
		}
		log.Printf(
			"device index=%d name=%q channels=%d freq=`%d Hz` format=0x%x",
			i, name, spec.Channels, spec.Freq, spec.Format,
		)
	}
}
