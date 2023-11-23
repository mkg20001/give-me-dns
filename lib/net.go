package lib

import (
	"context"
	"fmt"
	"github.com/getsentry/sentry-go"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

func ProvideNet(config *NetConfig, store *Store, ctx context.Context, errChan chan<- error) {
	go func() {
		listen, err := net.Listen("tcp", config.Address+":"+strconv.Itoa(int(config.Port)))
		if err != nil {
			errChan <- err
			return
		}

		log.Printf("TCP listens on %s:%d\n", config.Address, config.Port)

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

					entry, dnsName, err := store.AddEntry(remoteAddr)
					if err != nil {
						sentry.CaptureException(err)
						log.Printf("Failed to add entry: %s", err)
						return "Failed to add entry.\n"
					}

					log.Printf("New entry %s - IP %s\n", dnsName, remoteAddr)
					return fmt.Sprintf("Address: %s\nDNS Name: %s\nValid for %s\nExpires %s\n", remoteAddr, dnsName, store.TTL().String(), entry.Expires.Format(time.RFC3339))
				}()

				_, err := conn.Write([]byte(responseStr))
				if err != nil {
					sentry.CaptureException(err)
				}
			}()
		}
	}()
}
