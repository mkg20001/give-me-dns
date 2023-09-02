package main

import (
	"context"
	"github.com/mkg20001/give-me-dns/lib"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	var wg2 sync.WaitGroup
	wg := &wg2

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	wg.Add(3)

	config, err := lib.ReadConfig(os.Args[1])
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("Starting give-me-dns...\n")
	err = Init(config, ctx)
	if err != nil {
		log.Fatalln(err)
	}
	<-ctx.Done()
}
