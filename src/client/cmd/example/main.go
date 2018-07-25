package main

import (
	"client"
	"client/api/v1"
	"client/bots"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	upwingo := v1.NewUpwingo(v1.Config{APIKey: "XXX"})

	env := client.Client{
		API:       upwingo,
		Bot:       &bots.BotSimple{},
		Reconnect: true,
	}.Run()

	<-terminate(func(sig os.Signal) {
		env.Stop()
	})
}

func terminate(fn func(os.Signal)) <-chan struct{} {
	signals := make(chan os.Signal, 1)
	done := make(chan struct{})

	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-signals
		fn(sig)
		time.Sleep(time.Second)
		done <- struct{}{}
	}()

	return done
}
