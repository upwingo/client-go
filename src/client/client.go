package client

import (
	"client/api"
	"client/bots"
	"log"
)

type Client struct {
	API       api.Binary
	Bot       bots.Bot
	Reconnect bool
}

func (c *Client) Run() *Client {
	c.Bot.SetAPI(c.API)
	c.Bot.SetStopped(false)

	c.Bot.OnInit()

	if c.Bot.IsStopped() {
		return c
	}

	c.API.TickerStart(func() {
		for _, channel := range c.Bot.GetChannels() {
			c.API.TickerWatch(channel, func(data interface{}) {
				c.Bot.OnTick(data)
				if c.Bot.IsStopped() {
					c.Stop()
				}
			})
		}

	}, func(err error) {
		log.Printf("client: %v", err)

	}, func(err error) {
		log.Printf("client: %v", err)

		if c.Reconnect {
			c.API.TickerReconnect()
		}
	})

	return c
}

func (c *Client) Stop() *Client {
	c.API.TickerStop()
	return c
}
