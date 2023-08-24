package lib

import (
	"context"
	"fmt"
	"github.com/miekg/dns"
	"log"
	"strconv"
	"sync"
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
			lastBlock := q.Name[labelIndexes[0]:labelIndexes[1]]
			ip, err := store.ResolveEntry(lastBlock)
			if err != nil {
				return
			}
			if ip != "" {
				rr, err := dns.NewRR(fmt.Sprintf("%s AAAA %s", q.Name, ip))
				if err == nil {
					m.Answer = append(m.Answer, rr)
				}
			}
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

func ProvideDNS(config *Config, store *Store, ctx context.Context, wg *sync.WaitGroup) error {
	// attach request handler func
	dns.HandleFunc(config.Domain+".", func(w dns.ResponseWriter, r *dns.Msg) {
		handleDnsRequest(w, r, store)
	})

	// start server
	server := &dns.Server{Addr: config.DNSAddress + ":" + strconv.Itoa(int(config.DNSPort)), Net: "udp"}

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			fmt.Printf("err")
		}
		defer wg.Done()
	}()

	go func() {
		<-ctx.Done()
		server.Shutdown()
	}()

	return nil
}
