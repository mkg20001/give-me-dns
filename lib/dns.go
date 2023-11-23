package lib

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"encoding/base64"
	"github.com/getsentry/sentry-go"
	"github.com/miekg/dns"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func parseDNSQuery(r *dns.Msg, m *dns.Msg, store *Store, s *DNSSECSigner) {
	m.Authoritative = true
	shouldSign := false

	main := store.Config.Domain + "."

	if r.IsEdns0() != nil {
		if r.IsEdns0().Do() {
			shouldSign = true
		}
		m.SetEdns0(4096, shouldSign)
	}

	for _, q := range m.Question {
		ismain := strings.ToLower(q.Name) == main

		log.Printf("Question %s", q.String())

		soa := new(dns.SOA)
		soa.Hdr = dns.RR_Header{
			Name:   q.Name,
			Rrtype: dns.TypeSOA,
			Class:  dns.ClassINET,
			Ttl:    3600,
		}

		soa.Mbox = store.Config.DNSMNAME
		soa.Ns = store.Config.DNSNS[0]
		soa.Minttl = 3600
		soa.Refresh = 1
		soa.Retry = 1
		soa.Serial = store.GetSerial()
		soa.Expire = 1

		switch q.Qtype {
		case dns.TypeDNSKEY:
			if ismain {
				log.Printf("A DNSKEY")
				m.Answer = append(m.Answer, s.GetDNSKEY())
			}
		case dns.TypeDS:
			if ismain {
				log.Printf("A DS")
				m.Answer = append(m.Answer, s.GetDS())
			}
		case dns.TypeNS:
			if ismain {
				log.Printf("A NS")
				for _, ns := range store.Config.DNSNS {
					nsrr := new(dns.NS)
					nsrr.Ns = ns
					nsrr.Hdr = dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeNS,
						Class:  dns.ClassINET,
						Ttl:    3600,
					}
					m.Answer = append(m.Answer, nsrr)
				}
			}
		case dns.TypeSOA:
			if ismain && q.Qtype == dns.TypeSOA {
				log.Printf("A SOA")
				m.Answer = append(m.Answer, soa)
			}
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
		}

		if len(m.Answer) > 0 {
			if shouldSign {
				rrsig, err := s.Sign(m.Answer)
				if err != nil {
					sentry.CaptureException(err)
					log.Printf("dnssec err: %s", err)
					return
				}
				m.Answer = append(m.Answer, rrsig)
			}
		} else {
			r := new(dns.SOA)
			r.Hdr = dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeSOA,
				Class:  dns.ClassINET,
				Ttl:    3600,
			}

			r.Mbox = store.Config.DNSMNAME
			r.Ns = store.Config.DNSNS[0]
			r.Minttl = 3600
			r.Refresh = 1
			r.Retry = 1
			r.Serial = store.GetSerial()
			r.Expire = 1

			m.Ns = append(m.Ns, r)

			if shouldSign {
				nsec := &dns.NSEC{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeNSEC,
						Class:  dns.ClassINET,
						Ttl:    3600,
					},
					NextDomain: "\000" + "." + q.Name,
					TypeBitMap: []uint16{dns.TypeNS, dns.TypeSOA},
				}
				m.Ns = append(m.Ns, nsec)

				rrsig, err := s.Sign([]dns.RR{r})
				if err != nil {
					sentry.CaptureException(err)
					log.Printf("dnssec err: %s", err)
					return
				}
				m.Ns = append(m.Ns, rrsig)

				rrsig2, err := s.Sign([]dns.RR{nsec})
				if err != nil {
					sentry.CaptureException(err)
					log.Printf("dnssec err: %s", err)
					return
				}
				m.Ns = append(m.Ns, rrsig2)
			}
		}
	}
}

func handleDnsRequest(w dns.ResponseWriter, r *dns.Msg, store *Store, s *DNSSECSigner) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = false

	switch r.Opcode {
	case dns.OpcodeQuery:
		parseDNSQuery(r, m, store, s)
	}

	w.WriteMsg(m)
}

type DNSSECSigner struct {
	d      dns.DNSKEY
	signer crypto.Signer
	config *Config
}

func (s *DNSSECSigner) setupRecord() {
	s.d.Hdr = dns.RR_Header{
		Name:   s.config.Domain + ".",
		Rrtype: dns.TypeDNSKEY,
		Class:  dns.ClassINET,
		Ttl:    3600,
	}
	s.d.Protocol = 3 // DNSSEC
	s.d.Flags = 257  // ZONE, SEP
	s.d.Algorithm = dns.ECDSAP256SHA256
}

func (s *DNSSECSigner) Generate() (string, error) {
	s.setupRecord()

	key, err := s.d.Generate(256)
	if err != nil {
		return "", err
	}
	str := s.d.PrivateKeyString(key)

	_, err = s.d.ReadPrivateKey(strings.NewReader(str), "export")
	if err != nil {
		return "", err
	}

	signer := key.(*ecdsa.PrivateKey)
	s.signer = signer
	return base64.StdEncoding.EncodeToString([]byte(str + "PublicKey: " + s.d.PublicKey + "\n")), nil
}

var PubRe = regexp.MustCompile("PublicKey: (.*)\n")

func (s *DNSSECSigner) Load(str string) error {
	s.setupRecord()

	decoded, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return err
	}

	match := PubRe.FindSubmatch(decoded)
	if match == nil {
		return dns.ErrPrivKey
	}

	s.d.PublicKey = string(match[1])

	key, err := s.d.ReadPrivateKey(strings.NewReader(string(decoded)), "dnssec_key from config")
	if err != nil {
		return err
	}

	switch signer := key.(type) {
	case *ecdsa.PrivateKey:
		s.signer = signer
	case *rsa.PrivateKey:
		s.signer = signer
	default:
		return dns.ErrPrivKey
	}

	return nil
}

func (s *DNSSECSigner) GetDNSKEY() *dns.DNSKEY {
	return &s.d
}

func (s *DNSSECSigner) GetDS() *dns.DS {
	if s.signer == nil {
		return nil
	}

	ds := s.d.ToDS(2)
	ds.Hdr.Name = s.config.Domain + "."
	ds.Hdr.Ttl = uint32((time.Hour * 24 * 30).Seconds())

	return ds
}

func (s *DNSSECSigner) GetDSStr() string {
	ds := s.GetDS()
	if ds != nil {
		return ds.String()
	}

	return ""
}

func (s *DNSSECSigner) Sign(rr []dns.RR) (*dns.RRSIG, error) {
	if s.signer == nil {
		return nil, dns.ErrPrivKey
	}

	rrsig := new(dns.RRSIG)
	rrsig.Algorithm = s.d.Algorithm
	rrsig.KeyTag = s.d.KeyTag()
	rrsig.SignerName = s.config.Domain + "."
	rrsig.Inception = uint32(time.Now().Unix() - 3600)
	ttl := rr[0].Header().Ttl
	rrsig.Expiration = uint32(time.Now().Add(time.Duration(float64(time.Second)*float64(ttl)) + (time.Second * 3600)).Unix())
	rrsig.Hdr.Ttl = rr[0].Header().Ttl
	err := rrsig.Sign(s.signer, rr)
	if err != nil {
		return nil, err
	}
	return rrsig, nil
}

func ProvideDNS(config *Config, store *Store, ctx context.Context, errChan chan<- error) {
	// prepare dnssec
	s := &DNSSECSigner{
		config: config,
	}
	if config.DNSSECKey == "" {
		keyexport, err := s.Generate()
		if err != nil {
			errChan <- err
			return
		}
		log.Printf("No DNSSEC key was provided. Please add the following into your config:\n")
		log.Printf("  dnssec_key: \"%s\"\n", keyexport)
	} else {
		err := s.Load(config.DNSSECKey)
		if err != nil {
			errChan <- err
			return
		}
	}

	log.Printf("DS Record: %s\n", s.GetDSStr())

	// attach request handler func
	mux := dns.NewServeMux()
	mux.HandleFunc(config.Domain+".", func(w dns.ResponseWriter, r *dns.Msg) {
		handleDnsRequest(w, r, store, s)
	})

	// create servers
	serverTcp := &dns.Server{
		Addr:      config.DNSAddress + ":" + strconv.Itoa(int(config.DNSPort)),
		Net:       "tcp",
		Handler:   mux,
		ReusePort: true,
	}
	serverUdp := &dns.Server{
		Addr:      config.DNSAddress + ":" + strconv.Itoa(int(config.DNSPort)),
		Net:       "udp",
		Handler:   mux,
		UDPSize:   65535,
		ReusePort: true,
	}

	go func() {
		log.Printf("DNS (tcp) listens on %s:%d\n", config.DNSAddress, config.DNSPort)
		err := serverTcp.ListenAndServe()
		if err != nil {
			errChan <- err
		}
	}()

	go func() {
		log.Printf("DNS (udp) listens on %s:%d\n", config.DNSAddress, config.DNSPort)
		err := serverUdp.ListenAndServe()
		if err != nil {
			errChan <- err
		}
	}()

	go func() {
		<-ctx.Done()
		err := serverTcp.Shutdown()
		if err != nil {
			errChan <- err
		}
		err = serverUdp.Shutdown()
		if err != nil {
			errChan <- err
		}
	}()
}
