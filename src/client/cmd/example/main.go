package main

import (
	"client"
	"client/api/v1"
	"client/bots"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var apiKey = flag.String("k", "", "API key")

func main() {
	flag.Parse()

	if *apiKey == "" {
		log.Fatal("apiKey undefined")
	}

	upwingo := v1.NewUpwingo(v1.Config{APIKey: *apiKey})

	env := client.Client{
		API:       upwingo,
		Bot:       &bots.BotSimple{},
		Reconnect: true,
	}
	env.Run()

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
