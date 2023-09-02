package lib

import (
	"context"
	"fmt"
	"github.com/getsentry/sentry-go"
	"log"
	"net"
	"strconv"
	"strings"
)

func ProvideNet(config *Config, store *Store, ctx context.Context, errChan chan<- error) {
	go func() {
		listen, err := net.Listen("tcp", config.NetAddress+":"+strconv.Itoa(int(config.NetPort)))
		if err != nil {
			errChan <- err
			return
		}

		fmt.Printf("TCP listens on %s:%d\n", config.NetAddress, config.NetPort)

		go func() {
			<-ctx.Done()
			err := listen.Close()
			if err != nil {
				errChan <- err
			}
		}()

		for {
			conn, err := listen.Accept()

			if err != nil {
				break
			}

			go func() {
				defer func(conn net.Conn) {
					err := conn.Close()
					if err != nil {
						sentry.CaptureException(err)
					}
				}(conn)

				responseStr := func() string {
					remoteAddr := conn.RemoteAddr().(*net.TCPAddr).IP
					if len(remoteAddr) == 4 || strings.Contains(remoteAddr.String(), ".") {
						return "IPv4 not supported\n"
					}

					id, err := store.AddEntry(remoteAddr)
					if err != nil {
						sentry.CaptureException(err)
						log.Printf("Failed to add entry: %s", err)
						return "Failed to add entry.\n"
					}

					dnsName := id + "." + config.Domain
					log.Printf("New entry %s - IP %s\n", dnsName, remoteAddr)
					return fmt.Sprintf("Address: %s\nDNS Name: %s\nValid for %s\n", remoteAddr, dnsName, config.TTL.String())
				}()

				_, err := conn.Write([]byte(responseStr))
				if err != nil {
					sentry.CaptureException(err)
				}
			}()
		}
	}()
}
