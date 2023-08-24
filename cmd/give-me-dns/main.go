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

	wg.Add(2)

	log.Printf("Starting give-me-dns...\n")
	cleanup, err := Init(ctx, wg, os.Args[1])
	if err != nil {
		log.Fatalln(err)
	}
	wg.Wait()
	defer cleanup()
}
