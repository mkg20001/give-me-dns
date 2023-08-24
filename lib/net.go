package lib

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
)

func ProvideNet(config *Config, store *Store, ctx context.Context, wg *sync.WaitGroup) error {
	e := make(chan error)

	go func() {
		listen, err := net.Listen("tcp", config.NetAddress+":"+strconv.Itoa(int(config.NetPort)))
		if err != nil {
			e <- err
			return
		}

		go func() {
			<-ctx.Done()
			listen.Close()
		}()
		e <- nil

		defer wg.Done()

		for {
			conn, err := listen.Accept()

			if err != nil {
				break
			}

			go func() {
				defer conn.Close()

				remoteAddr := conn.RemoteAddr().String()
				id, err := store.AddEntry(remoteAddr)
				var responseStr string
				if err != nil {
					responseStr = "Failed to add entry.\n"
				} else {
					dnsName := id + "." + config.Domain
					log.Printf("New entry %s - IP %s\n", dnsName, remoteAddr)
					responseStr = fmt.Sprintf("Remote Address: %s\nDNS Name: %s\nValid for %s\n", remoteAddr, dnsName, config.TTL.String())
				}

				conn.Write([]byte(responseStr))
			}()
		}
	}()

	return <-e
}
