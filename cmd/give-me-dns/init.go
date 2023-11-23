package main

import (
	"context"
	"github.com/getsentry/sentry-go"
	"github.com/mkg20001/give-me-dns/lib"
	"github.com/mkg20001/give-me-dns/lib/idprov"
	"log"
	"time"
)

func Init(config *lib.Config, _ctx context.Context) error {
	ctx, cancel := context.WithCancelCause(_ctx)

	log.Printf("Domain %s\n", config.Store.Domain)

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

	var idProv []idprov.IDProv

	if config.Provider.PWordlistID.Enable {
		wordlist, err := idprov.ProvideWordlistID()
		if err != nil {
			return err
		}
		idProv = append(idProv, wordlist)
	}

	if config.Provider.PRandomID.Enable {
		idProv = append(idProv, idprov.ProvideRandomID(config.Provider.PRandomID.IDLen))
	}

	err, cleanStore, store := lib.ProvideStore(&config.Store, idProv)
	if err != nil {
		cancel(err)
		sentry.CaptureException(err)
		return err
	}

	go func() {
		lib.ProvideDNS(&config.DNS, store, ctx, errChan)
		lib.ProvideNet(&config.Net, store, ctx, errChan)
		lib.ProvideHTTP(&config.HTTP, store, ctx, errChan)
	}()

	go func() {
		canceled := false
		for err := range errChan {
			log.Printf("Fatal: %s\n", err)
			if !canceled {
				cancel(err)
				canceled = true
			}
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
