package bots

import (
	"client/api"
)

type Bot interface {
	OnInit()
	OnTick(data interface{})

	SetAPI(api api.Binary)
	GetChannels() []string
	IsStopped() bool
	SetStopped(stopped bool)

	api() api.Binary
	stop()
}

type Base struct {
	apiInst  api.Binary
	channels []string
	stopped  bool
}

func (b *Base) SetAPI(api api.Binary) {
	b.apiInst = api
}

func (b *Base) subscribe(channel string) {
	b.channels = append(b.channels, channel)
}

func (b *Base) GetChannels() []string {
	return b.channels
}

func (b *Base) IsStopped() bool {
	return b.stopped
}

func (b *Base) SetStopped(stopped bool) {
	b.stopped = stopped
}

func (b *Base) api() api.Binary {
	return b.apiInst
}

func (b *Base) stop() {
	b.stopped = true
}

type OHLCVT struct {
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
	Time   int64
}
