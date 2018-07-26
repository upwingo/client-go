package bots

import (
	"fmt"
	"log"
	"math"
	"strings"
)

const bsCap = 5
const bsTrend = 2

type BotSimple struct {
	Base

	Exchange  string
	Symbol    string
	Timeframe int64
	Limit     string
	Currency  string
	Amount    float64

	tableId string
	balance float64

	candles             map[int64]*Candle
	currentCandleTime   int64
	lastOrderCandleTime int64
}

func (b *BotSimple) OnInit() {
	b.Currency = strings.ToUpper(b.Currency)

	channel := strings.ToUpper(fmt.Sprintf("CANDLES--%s-%s--%d", b.Exchange, b.Symbol, b.Timeframe))
	b.tableId = strings.ToLower(fmt.Sprintf("%s--%s--%d--%s--single:", b.Exchange, b.Symbol, b.Timeframe, b.Limit)) + b.Currency + "--pvp"

	balances, err := b.api().GetBalance()
	if err != nil {
		log.Print(err)
		b.stop()
		return
	}

	b.balance = balances.Free[b.Currency]

	log.Printf("balance: %f", b.balance)

	b.subscribe(channel)
}

// concurrent tick handler
func (b *BotSimple) OnTick(data interface{}) {
	if b.Amount > b.balance {
		log.Print("insufficient funds")
		b.stop()
		return
	}

	if !b.updateCandles(data) {
		return
	}

	command := b.condition()
	if command == 0 {
		return
	}

	order := map[string]string{
		"table_id": b.tableId,
		"type":     fmt.Sprintf("%d", command),
		"amount":   fmt.Sprintf("%f", b.Amount),
		"currency": b.Currency,
	}

	orderId, balances, err := b.api().OrderCreate(order)
	if err != nil {
		log.Print(err)
		return
	}

	b.balance = balances.Free[b.Currency]

	b.lastOrderCandleTime = b.currentCandleTime

	log.Printf(
		"order %s created [%s %s %s %s] balance: %f",
		orderId,
		order["table_id"],
		order["amount"],
		order["currency"],
		order["type"],
		b.balance,
	)
}

//
//                    up | up | -> | up
//                down | down | -> | down
//
//          up | up | up | up | -> | down
//  down | down | down | down | -> | up
//
func (b *BotSimple) condition() int {
	if b.currentCandleTime <= b.lastOrderCandleTime {
		return 0
	}

	time := b.currentCandleTime - b.Timeframe
	candle := b.candles[time]
	if candle == nil {
		return 0
	}

	diff := candle.Close - candle.Open
	if math.Abs(diff) < POINT {
		return 0
	}

	dir := 1
	if diff < 0 {
		dir = -1
	}

	command := 0

	for i := 2; i < bsCap; i++ {
		time -= b.Timeframe
		candle := b.candles[time]
		if candle == nil {
			return command
		}

		if (candle.Close-candle.Open)*float64(dir) < POINT {
			return command
		}

		if i == bsTrend {
			command = dir
		}
	}

	command = -command

	return command
}

func (b *BotSimple) updateCandles(data interface{}) bool {
	var csvs []interface{}
	if data, ok := data.(map[string]interface{}); ok {
		if data, ok := data["candles"]; ok {
			if csvs, ok = data.([]interface{}); !ok {
				return false
			}
		}
	}

	if b.candles == nil {
		b.candles = make(map[int64]*Candle)
	}

	for _, csv := range csvs {
		candle := GetCandle(csv.(string))
		if !candle.IsComplete() {
			continue
		}

		b.candles[candle.Time] = &candle

		if candle.Time > b.currentCandleTime {
			b.currentCandleTime = candle.Time
		}
	}

	for time := range b.candles {
		if time <= b.currentCandleTime-bsCap*b.Timeframe {
			delete(b.candles, time)
		}
	}

	return true
}
