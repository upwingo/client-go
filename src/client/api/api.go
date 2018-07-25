package api

type Balance struct {
	Free   map[string]float64
	InGame map[string]float64
}

type Binary interface {
	TickerStart(onSuccess func(), onError func(err error), onDisconnect func(err error)) error
	TickerWatch(channel string, onTick func(data interface{}))
	TickerStop()
	TickerReconnect()
	GetTablesList() ([]byte, error)
	GetNextRoundInfo(map[string]string) ([]byte, error)
	GetHistory(map[string]string) ([]byte, error)
	OrderCreate(map[string]string) (string, Balance, error)
	OrderCancel(string) (Balance, error)
	GetBalance() (Balance, error)
}
