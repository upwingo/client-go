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
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
	config       Config
	socket       *scclient.Client
	stopWatchers []chan struct{}
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
	if u.socket != nil {
		return errors.New("upwingo: ticker already started")
	}

	client := scclient.New(u.config.WSHost + fmt.Sprintf(":%d/socketcluster/", u.config.WSPort))
	client.SetBasicListener(func(client scclient.Client) {
		if onSuccess != nil {
			onSuccess()
		}
	}, func(client scclient.Client, err error) {
		if onError != nil {
			onError(err)
		}
	}, func(client scclient.Client, err error) {
		for _, ch := range u.stopWatchers {
			go func() { ch <- struct{}{} }()
		}
		u.stopWatchers = u.stopWatchers[:0]

		if onDisconnect != nil {
			onDisconnect(err)
		}
	})
	client.Connect()

	u.socket = &client

	return nil
}

func (u *Upwingo) TickerWatch(channel string, onTick func(data interface{})) {
	if u.socket == nil {
		u.TickerStart(nil, nil, nil)
	}

	dataChan := make(chan interface{}, 1)

	u.socket.Subscribe(channel)
	u.socket.OnChannel(channel, func(eventName string, data interface{}) {
		select {
		case dataChan <- data:
		case <-dataChan:
			dataChan <- data
		}
	})

	if onTick == nil {
		return
	}

	stopChan := make(chan struct{}, 1)

	go func() {
		for {
			select {
			case data, ok := <-dataChan:
				if !ok {
					return
				}
				onTick(data)
			case <-stopChan:
				return
			}
		}
	}()

	u.stopWatchers = append(u.stopWatchers, stopChan)
}

func (u *Upwingo) TickerStop() {
	if u.socket != nil {
		u.socket.Disconnect()
		u.socket = nil
	}
}

func (u *Upwingo) TickerReconnect() {
	if u.socket != nil && !u.socket.IsConnected() {
		u.socket.Connect()
	}
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

	client := http.Client{Timeout: time.Duration(30) * time.Second}

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

	if rawMap, ok := raw.(map[string]interface{}); ok {
		if free, ok := rawMap["FREE"]; ok {
			if freeMap, ok := free.(map[string]interface{}); ok {
				balance.Free = make(map[string]float64)
				for currency, value := range freeMap {
					balance.Free[currency], _ = strconv.ParseFloat(value.(string), 64)
				}
			}
		}
		if inGame, ok := rawMap["IN GAME"]; ok {
			if inGameMap, ok := inGame.(map[string]interface{}); ok {
				balance.InGame = make(map[string]float64)
				for currency, value := range inGameMap {
					balance.InGame[currency], _ = strconv.ParseFloat(value.(string), 64)
				}
			}
		}
	}

	return balance
}
