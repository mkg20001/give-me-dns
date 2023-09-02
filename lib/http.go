package lib

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/hoisie/mustache"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//go:embed index.html
var assetFS embed.FS

func jsonResponse(a JSONReply, writer http.ResponseWriter) {
	b, err := json.Marshal(a)
	if err != nil {
		sentry.CaptureException(err)
		return
	}

	_, err = writer.Write(b)
	if err != nil {
		sentry.CaptureException(err)
		return
	}
}

type JSONReply struct {
	OK  bool             `json:"ok"`
	Err string           `json:"error,omitempty"`
	Res interface{ any } `json:"res"`
}

type JSONGet struct {
	HasDNS  bool   `json:"has_dns"`
	TTL     string `json:"ttl"`
	DNSName string `json:"dns_name,omitempty"`
	Expires string `json:"expires,omitempty"`
	Address net.IP `json:"address"`
}

func getIP(w http.ResponseWriter, req *http.Request) (net.IP, error) {
	forward := req.Header.Get("X-Forwarded-For")
	if forward != "" {
		ips := strings.Split(forward, ",")
		ip := ips[0]
		parsed := net.ParseIP(ip)
		if parsed == nil {
			return nil, net.InvalidAddrError(ip)
		}

		return parsed, nil
	}

	ip, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return nil, net.InvalidAddrError(req.RemoteAddr)
	}

	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil, net.InvalidAddrError(ip)
	}

	return parsed, nil
}

func getInfo(w http.ResponseWriter, req *http.Request, store *Store) (*JSONGet, error) {
	ip, err := getIP(w, req)
	if err != nil {
		return nil, err
	}

	info := &JSONGet{
		Address: ip,
		TTL:     store.Config.TTL.String(),
	}

	entry, id, err := store.ResolveIP(ip)
	if err != nil {
		return nil, err
	}
	if id != "" {
		info.HasDNS = true
		info.Expires = entry.Expires.Format(time.RFC3339)
		info.DNSName = id
	}

	return info, err
}

const FailedToGetInfo = "Failed to get information about client"
const FailedToAddEntry = "Failed to add entry"

func ProvideHTTP(config *Config, store *Store, ctx context.Context, errChan chan<- error) {
	file, err := assetFS.ReadFile("index.html")
	if err != nil {
		errChan <- err
		return
	}

	template, err := mustache.ParseString(string(file))
	if err != nil {
		errChan <- err
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case "GET":
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			info, err := getInfo(writer, request, store)
			if err != nil {
				sentry.CaptureException(err)
				log.Printf("HTTP err: %s\n", err)
				fmt.Fprint(writer, template.Render(&JSONReply{
					Err: FailedToGetInfo,
				}))
			} else {
				fmt.Fprint(writer, template.Render(&JSONReply{
					Res: info,
					OK:  true,
				}))
			}
		case "POST":
			ip, err := getIP(writer, request)
			if err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}

			_, _, err = store.AddEntry(ip)
			if err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}

			// switch to GET
			http.Redirect(writer, request, "/", http.StatusSeeOther)
		default:
			writer.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/json", func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case "GET":
			info, err := getInfo(writer, request, store)
			if err != nil {
				jsonResponse(JSONReply{
					Err: FailedToGetInfo,
				}, writer)
				return
			}

			jsonResponse(JSONReply{
				OK:  true,
				Res: info,
			}, writer)
		case "POST":
			ip, err := getIP(writer, request)
			if err != nil {
				writer.WriteHeader(http.StatusBadRequest)
				jsonResponse(JSONReply{
					Err: FailedToGetInfo,
				}, writer)
				return
			}

			_, _, err = store.AddEntry(ip)
			if err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				jsonResponse(JSONReply{
					Err: FailedToAddEntry,
				}, writer)
				return
			}

			// switch to GET
			http.Redirect(writer, request, "/json", http.StatusSeeOther)
		default:
			writer.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	server := &http.Server{
		Addr:    config.HTTPAddress + ":" + strconv.Itoa(int(config.HTTPPort)),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		err := server.Close()
		if err != nil {
			errChan <- err
		}
	}()

	go func() {
		log.Printf("HTTP listens on %s:%d\n", config.HTTPAddress, config.HTTPPort)

		err = server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()
}
