//go:build wireinject
// +build wireinject

package main

import (
	"context"
	"github.com/google/wire"
	"github.com/mkg20001/give-me-dns/lib"
	"sync"
)

func Init(ctx context.Context, wg *sync.WaitGroup, config string) (func(), error) {
	wire.Build(
		lib.ProvideConfig,
		lib.ProvideStore,
		lib.ProvideNet,
		lib.ProvideDNS,
	)

	return nil, nil
}
