package main

import "time"

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
