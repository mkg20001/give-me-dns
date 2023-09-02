package main

import (
	"context"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/mkg20001/give-me-dns/lib"
	"time"
)

func Init(config *lib.Config, _ctx context.Context) error {
	ctx, cancel := context.WithCancelCause(_ctx)

	err := sentry.Init(sentry.ClientOptions{
		Dsn: config.SentryDSN,
		// Enable printing of SDK debug messages.
		// Useful when getting started or trying to figure something out.
		Debug: true,
	})
	if err != nil {
		return err
	}
	defer sentry.Flush(10 * time.Second)

	errChan := make(chan error)

	err, cleanStore, store := lib.ProvideStore(config)
	if err != nil {
		cancel(err)
		sentry.CaptureException(err)
		return err
	}

	lib.ProvideDNS(config, store, ctx, errChan)
	lib.ProvideNet(config, store, ctx, errChan)
	lib.ProvideHTTP(config, store, ctx, errChan)

	go func() {
		canceled := false
		for err := range errChan {
			fmt.Printf("Fatal: %s\n", err)
			if !canceled {
				cancel(err)
				canceled = true
			}
			fmt.Printf("Postc")
			sentry.CaptureException(err)
		}
	}()

	<-ctx.Done()
	err = cleanStore()
	if err != nil {
		errChan <- err
	}

	return nil
}
