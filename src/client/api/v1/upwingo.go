package v1

import (
	"bytes"
	"client/api"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sacOO7/socketcluster-client-go/scclient"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type response interface {
	GetCode() int
}

type responseCode struct {
	Code     int         `json:"code"`
	Status   string      `json:"status"`
	Messages interface{} `json:"messages"`
	Errors   interface{} `json:"errors"`
}

func (r *responseCode) GetCode() int {
	return r.Code
}

type Config struct {
	WSHost string
	WSPort int
	Host   string
	APIKey string
}

type Upwingo struct {
	config Config

	socket   *scclient.Client
	socketMu sync.Mutex

	stopWatchers []chan struct{}
	ping         chan struct{}
	stopsPingMu  sync.Mutex
}

func NewUpwingo(config Config) *Upwingo {
	if config.WSHost == "" {
		config.WSHost = "wss://ws.upwingo.com"
	}

	if config.WSPort == 0 {
		config.WSPort = 443
	}

	if config.Host == "" {
		config.Host = "https://api.upwingo.com"
	}

	return &Upwingo{config: config}
}

func (u *Upwingo) TickerStart(onSuccess func(), onError func(err error), onDisconnect func(err error)) error {
	u.socketMu.Lock()
	defer u.socketMu.Unlock()

	if u.socket != nil {
		return errors.New("upwingo: ticker already started")
	}

	client := scclient.New(u.config.WSHost + fmt.Sprintf(":%d/socketcluster/", u.config.WSPort))
	u.socket = &client
	u.socket.SetBasicListener(func(client scclient.Client) {
		if onSuccess != nil {
			go onSuccess()
		}
	}, func(client scclient.Client, err error) {
		if onError != nil {
			go onError(err)
		}
	}, func(client scclient.Client, err error) {
		u.stopsPingMu.Lock()

		for _, ch := range u.stopWatchers {
			go func(ch chan struct{}) { ch <- struct{}{} }(ch)
		}
		u.stopWatchers = u.stopWatchers[:0]
		u.ping = nil

		u.stopsPingMu.Unlock()

		if onDisconnect != nil {
			go onDisconnect(err)
		}
	})
	u.socket.Connect()

	return nil
}

func (u *Upwingo) TickerWatch(channel string, onTick func(data interface{})) {
	if onTick == nil {
		return
	}

	u.socketMu.Lock()
	defer u.socketMu.Unlock()

	if u.socket == nil {
		return
	}

	dataChan := make(chan interface{}, 1)

	u.stopsPingMu.Lock()

	stopChan := make(chan struct{}, 1)
	go func() {
		log.Printf("upwingo: new watcher %s started", channel)
		for {
			select {
			case data := <-dataChan:
				onTick(data)
			case <-stopChan:
				log.Printf("upwingo: watcher %s stopped", channel)
				return
			}
		}
	}()
	u.stopWatchers = append(u.stopWatchers, stopChan)

	if u.ping == nil {
		u.ping = make(chan struct{}, 1)
		stopChan := make(chan struct{}, 1)
		go func(ping chan struct{}) {
			log.Print("upwingo: new ping started")
			for {
				select {
				case <-ping:
				case <-stopChan:
					log.Print("upwingo: ping stopped")
					return
				case <-time.After(2 * time.Minute):
					log.Print("upwingo: watchers idle")
					go u.tickerDisconnect()
				}
			}
		}(u.ping)
		u.stopWatchers = append(u.stopWatchers, stopChan)
	}

	u.stopsPingMu.Unlock()

	ping := u.ping

	u.socket.Subscribe(channel)
	u.socket.OnChannel(channel, func(eventName string, data interface{}) {
		select {
		case dataChan <- data:
		case <-dataChan:
			dataChan <- data
		}
		ping <- struct{}{}
	})
}

func (u *Upwingo) TickerStop() {
	var socket *scclient.Client

	u.socketMu.Lock()
	if u.socket != nil {
		socket = u.socket
		u.socket = nil
	}
	u.socketMu.Unlock()

	if socket != nil && socket.IsConnected() {
		socket.Disconnect()
	}
}

func (u *Upwingo) TickerReconnect(timeout time.Duration) {
	defer u.socketMu.Unlock()

	u.socketMu.Lock()
	if u.socket == nil || u.socket.IsConnected() {
		return
	}

	u.socketMu.Unlock()

	log.Print("upwingo: wait for reconnect")
	time.Sleep(timeout)

	u.socketMu.Lock()
	if u.socket != nil && !u.socket.IsConnected() {
		u.socket.Connect()
	}
}

func (u *Upwingo) tickerDisconnect() {
	u.socketMu.Lock()
	if u.socket != nil && u.socket.IsConnected() {
		u.socket.Disconnect()
	}
	u.socketMu.Unlock()
}

func (u *Upwingo) GetTablesList() ([]byte, error) {
	return u.get("/v1/binary/tables", nil, &responseCode{})
}

func (u *Upwingo) GetNextRoundInfo(params map[string]string) ([]byte, error) {
	return u.get("/v1/binary/round", params, &responseCode{})
}

func (u *Upwingo) GetHistory(params map[string]string) ([]byte, error) {
	return u.get("/v1/binary/history", params, &responseCode{})
}

func (u *Upwingo) OrderCreate(params map[string]string) (string, api.Balance, error) {
	var data struct {
		responseCode
		Data    interface{} `json:"data"`
		Balance interface{} `json:"balance"`
	}

	_, err := u.post("/v1/binary/order", params, &data)

	orderId := ""
	if order, ok := data.Data.(map[string]interface{}); ok {
		if value, ok := order["order_id"]; ok {
			switch value.(type) {
			case string:
				orderId = strings.TrimSpace(value.(string))
			case int64, int32, int:
				orderId = fmt.Sprintf("%d", value)
			case float64, float32:
				orderId = fmt.Sprintf("%.0f", value)
			}
		}
	}

	return orderId, decodeBalance(data.Balance), err
}

func (u *Upwingo) OrderCancel(orderId string) (api.Balance, error) {
	var data struct {
		responseCode
		Balance interface{} `json:"balance"`
	}

	_, err := u.post(fmt.Sprintf("/v1/binary/order/%s/cancel", orderId), nil, &data)

	return decodeBalance(data.Balance), err
}

func (u *Upwingo) GetBalance() (api.Balance, error) {
	var data struct {
		responseCode
		Data interface{} `json:"data"`
	}

	_, err := u.get("/v1/balance", nil, &data)

	return decodeBalance(data.Data), err
}

func (u *Upwingo) get(uri string, params map[string]string, data response) ([]byte, error) {
	return u.request("GET", uri, params, data)
}

func (u *Upwingo) post(uri string, params map[string]string, data response) ([]byte, error) {
	return u.request("POST", uri, params, data)
}

func (u *Upwingo) request(method string, uri string, params map[string]string, data response) ([]byte, error) {
	uri = u.config.Host + uri
	var body io.ReadWriter
	if params != nil {
		if method == "GET" {
			query := url.Values{}
			for key, value := range params {
				query.Set(key, value)
			}
			if q := query.Encode(); q != "" {
				uri += "?" + q
			}
		} else {
			body = new(bytes.Buffer)
			if err := json.NewEncoder(body).Encode(params); err != nil {
				return nil, err
			}
		}
	}

	req, err := http.NewRequest(method, uri, body)
	if err != nil || req == nil {
		return nil, fmt.Errorf("upwingo: cannot create request %s [ %v ]: %v", uri, params, err)
	}

	req.Header.Set("Authorization", "Bearer "+u.config.APIKey)
	if method != "GET" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := http.Client{Timeout: 30 * time.Second}

	resp, err := client.Do(req)
	if err != nil || resp == nil {
		return nil, fmt.Errorf("upwingo: cannot request %s [ %v ]: %v", uri, params, err)
	}

	defer resp.Body.Close()

	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(raw, data); err != nil {
		return raw, fmt.Errorf("upwingo: bad responce %s [ %v ]: %s", uri, params, raw)
	}

	if data.GetCode() != 200 {
		return raw, fmt.Errorf("upwingo: code %d %s [ %v ]: %s", data.GetCode(), uri, params, raw)
	}

	return raw, nil
}

func decodeBalance(raw interface{}) api.Balance {
	var balance api.Balance
	balance.Free = make(map[string]float64)
	balance.InGame = make(map[string]float64)

	if rawMap, ok := raw.(map[string]interface{}); ok {
		if free, ok := rawMap["FREE"]; ok {
			if freeMap, ok := free.(map[string]interface{}); ok {
				for currency, value := range freeMap {
					if amount, err := strconv.ParseFloat(value.(string), 64); err == nil {
						balance.Free[currency] = amount
					}
				}
			}
		}
		if inGame, ok := rawMap["IN GAME"]; ok {
			if inGameMap, ok := inGame.(map[string]interface{}); ok {
				for currency, value := range inGameMap {
					if amount, err := strconv.ParseFloat(value.(string), 64); err == nil {
						balance.InGame[currency] = amount
					}
				}
			}
		}
	}

	return balance
}
