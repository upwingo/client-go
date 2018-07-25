package bots

import "client/api"

type Bot interface {
	OnInit()
	OnTick(data interface{})

	SetAPI(api api.Binary)
	GetChannels() []string
	IsStopped() bool
	SetStopped(stopped bool)
	stop()
}

type Base struct {
	api      api.Binary
	channels []string
	stopped  bool
}

func (b *Base) SetAPI(api api.Binary) {
	b.api = api
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

func (b *Base) stop() {
	b.stopped = true
}
