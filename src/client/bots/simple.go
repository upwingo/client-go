package bots

import (
	"log"
)

type BotSimple struct {
	Base
}

func (b *BotSimple) OnInit() {
	b.subscribe("CANDLES--BINA-BTC_USDT--10")
}

// concurrent tick handler
func (b *BotSimple) OnTick(data interface{}) {
	log.Println(data)
}
