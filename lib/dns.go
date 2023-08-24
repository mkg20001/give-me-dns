package lib

import (
	"fmt"
	"github.com/miekg/dns"
	"log"
	"strconv"
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

func ProvideDNS(config *Config, store *Store) error {
	// attach request handler func
	dns.HandleFunc(config.Domain+".", func(w dns.ResponseWriter, r *dns.Msg) {
		handleDnsRequest(w, r, store)
	})

	// start server
	server := &dns.Server{Addr: config.DNSAddress + ":" + strconv.Itoa(int(config.DNSPort)), Net: "udp"}
	err := server.ListenAndServe()
	defer server.Shutdown()
	if err != nil {
		log.Fatalf("Failed to start server: %s\n ", err.Error())
		return err
	}

	return nil
}
