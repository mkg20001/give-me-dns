//go:build wireinject
// +build wireinject

package main

import (
	"github.com/google/wire"
	"github.com/mkg20001/give-me-dns/lib"
)

func Init(config string) error {
	wire.Build(
		lib.ProvideConfig,
		lib.ProvideStore,
		lib.ProvideDNS,
	)

	return nil
}
