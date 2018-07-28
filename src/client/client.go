package client

import (
	"client/api"
	"client/bots"
	"log"
	"time"
)

type Client struct {
	API       api.Binary
	Bot       bots.Bot
	Reconnect bool
}

func (c *Client) Run() *Client {
	c.Bot.SetAPI(c.API)
	c.Bot.SetStopped(false)

	channels := make([]string, 0, 1)
	c.Bot.SetSubscribe(func(channel string) {
		channels = append(channels, channel)
	})

	c.Bot.OnInit()

	if c.Bot.IsStopped() {
		return c
	}

	c.API.TickerStart(func() {
		log.Print("client: connected")

		for _, channel := range channels {
			c.API.TickerWatch(channel, func(data interface{}) {
				c.Bot.OnTick(data)
				if c.Bot.IsStopped() {
					c.Stop()
				}
			})
		}

	}, func(err error) {
		log.Printf("client: %v", err)

		if c.Reconnect {
			c.API.TickerReconnect(30 * time.Second)
		}

	}, func(err error) {
		if err != nil {
			log.Printf("client: disconnected: %v", err)
		} else {
			log.Print("client: disconnected")
		}

		if c.Reconnect {
			c.API.TickerReconnect(30 * time.Second)
		}
	})

	return c
}

func (c *Client) Stop() *Client {
	c.API.TickerStop()
	return c
}
