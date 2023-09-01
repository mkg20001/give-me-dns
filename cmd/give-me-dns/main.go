package main

import (
	"context"
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

	log.Printf("Starting give-me-dns...\n")
	err := Init(os.Args[1], ctx)
	if err != nil {
		log.Fatalln(err)
	}
	<-ctx.Done()
}
