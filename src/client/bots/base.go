package bots

import (
	"client/api"
	"strconv"
	"strings"
	"sync"
)

const POINT = 0.00000001

type Bot interface {
	OnInit()
	OnTick(data interface{}) // concurrent tick handler

	SetAPI(api api.Trade)
	SetSubscribe(fn func(string))
	IsStopped() bool
	SetStopped(stopped bool)

	api() api.Trade
	stop()
}

type Base struct {
	apiInst     api.Trade
	subscribeFn func(string)
	stopped     bool
	stoppedMu   sync.RWMutex
}

func (b *Base) SetAPI(api api.Trade) {
	b.apiInst = api
}

func (b *Base) SetSubscribe(fn func(string)) {
	b.subscribeFn = fn
}

func (b *Base) subscribe(channel string) {
	if b.subscribeFn != nil {
		b.subscribeFn(channel)
	}
}

func (b *Base) IsStopped() bool {
	b.stoppedMu.RLock()
	defer b.stoppedMu.RUnlock()

	return b.stopped
}

func (b *Base) SetStopped(stopped bool) {
	b.stoppedMu.Lock()
	defer b.stoppedMu.Unlock()

	b.stopped = stopped
}

func (b *Base) api() api.Trade {
	return b.apiInst
}

func (b *Base) stop() {
	b.SetStopped(true)
}

type Candle struct {
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
	Time   int64
}

func (c *Candle) IsComplete() bool {
	return c.Open > 0.0 &&
		c.High > 0.0 &&
		c.Low > 0.0 &&
		c.Close > 0.0 &&
		c.Volume >= 0.0 &&
		c.Time > 0
}

func GetCandle(csv string) Candle {
	var candle Candle

	fields := strings.Split(csv, ",")

	if v, err := strconv.ParseInt(strings.TrimSpace(fields[0]), 10, 64); err == nil {
		candle.Time = v
	}

	fields = fields[1:]

	var floats [5]float64
	for i, field := range fields {
		if i >= 5 {
			break
		}

		if v, err := strconv.ParseFloat(strings.TrimSpace(field), 64); err == nil {
			floats[i] = v
		}
	}

	candle.Open = floats[0]
	candle.High = floats[1]
	candle.Low = floats[2]
	candle.Close = floats[3]
	candle.Volume = floats[4]

	return candle
}
