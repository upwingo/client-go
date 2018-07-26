package api

import "time"

type Balance struct {
	Free   map[string]float64
	InGame map[string]float64
}

type Trade interface {
	GetTablesList() ([]byte, error)
	GetNextRoundInfo(map[string]string) ([]byte, error)
	GetHistory(map[string]string) ([]byte, error)
	OrderCreate(map[string]string) (string, Balance, error)
	OrderCancel(string) (Balance, error)
	GetBalance() (Balance, error)
}

type Binary interface {
	Trade
	TickerStart(onSuccess func(), onError func(err error), onDisconnect func(err error)) error
	TickerWatch(channel string, onTick func(data interface{}))
	TickerStop()
	TickerReconnect(timeout time.Duration)
}
