package bots

import "fmt"

type BotSimple struct {
	Base
}

func (b *BotSimple) OnInit() {
	b.subscribe("CANDLES--BINA-BTC_USDT--10")
}

func (b *BotSimple) OnTick(data interface{}) {
	fmt.Println(data)
}
