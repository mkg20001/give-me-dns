package lib

import (
	"context"
	"github.com/getsentry/sentry-go"
	"github.com/miekg/dns"
	"log"
	"strconv"
	"strings"
)

func parseDNSQuery(m *dns.Msg, store *Store) {
	for _, q := range m.Question {
		switch q.Qtype {
		case dns.TypeAAAA:
			log.Printf("Query for %s\n", q.Name)
			labelIndexes := dns.Split(q.Name)
			if len(labelIndexes) < 2 {
				return
			}
			lastBlock := strings.ToLower(q.Name)[labelIndexes[0] : labelIndexes[1]-1]
			ip, err := store.ResolveEntry(lastBlock)
			if err != nil {
				sentry.CaptureException(err)
				log.Printf("Failed to resolve: %s", err)
				return
			}
			if ip != nil {
				r := new(dns.AAAA)
				r.Hdr = dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    uint32(store.Config.TTL.Seconds()),
				}
				r.AAAA = ip

				log.Printf("Query for %s - Resolved %s\n", q.Name, ip)
				m.Answer = append(m.Answer, r)
			}
		default:
			r := new(dns.SOA)
			r.Hdr = dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeSOA,
				Class:  dns.ClassINET,
				Ttl:    3600,
			}

			r.Mbox = store.Config.DNSMNAME
			r.Ns = store.Config.DNSNS
			r.Minttl = 3600
			r.Refresh = 1
			r.Retry = 1
			r.Serial = store.GetSerial()
		}
	}
}

func handleDnsRequest(w dns.ResponseWriter, r *dns.Msg, store *Store) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = false

	switch r.Opcode {
	case dns.OpcodeQuery:
		parseDNSQuery(m, store)
	}

	w.WriteMsg(m)
}

func ProvideDNS(config *Config, store *Store, ctx context.Context, errChan chan<- error) {
	// attach request handler func
	dns.HandleFunc(config.Domain+".", func(w dns.ResponseWriter, r *dns.Msg) {
		handleDnsRequest(w, r, store)
	})

	// create server
	server := &dns.Server{Addr: config.DNSAddress + ":" + strconv.Itoa(int(config.DNSPort)), Net: "udp"}

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			errChan <- err
		}
	}()

	go func() {
		<-ctx.Done()
		err := server.Shutdown()
		if err != nil {
			errChan <- err
		}
	}()
}
