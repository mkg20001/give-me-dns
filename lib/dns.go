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
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func resolveDomain(q dns.Question, store *Store) net.IP {
	labelIndexes := dns.Split(q.Name)
	if len(labelIndexes) < 2 {
		return nil
	}
	lastBlock := strings.ToLower(q.Name)[labelIndexes[0] : labelIndexes[1]-1]
	ip, err := store.ResolveEntry(lastBlock)
	if err != nil {
		sentry.CaptureException(err)
		log.Printf("Failed to resolve: %s", err)
		return nil
	}
	if ip != nil {
		return ip
	}

	return nil
}

func parseDNSQuery(r *dns.Msg, m *dns.Msg, store *Store, config *DNSConfig, s *DNSSECSigner) {
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

		switch q.Qtype {
		case dns.TypeDNSKEY:
			if ismain {
				log.Printf("A DNSKEY")
				m.Answer = append(m.Answer, s.GetDNSKEY())
			}
		case dns.TypeNS:
			if ismain {
				log.Printf("A NS")
				for _, ns := range config.NS {
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
				m.Answer = append(m.Answer, s.GetSOA())
			}
		case dns.TypeAAAA:
			log.Printf("Query for %s\n", q.Name)
			ip := resolveDomain(q, store)
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
			soa := s.GetSOA()
			m.Ns = append(m.Ns, soa)

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

				rrsig, err := s.Sign([]dns.RR{soa})
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
			} else {
				ip := resolveDomain(q, store)
				if !ismain && ip == nil {
					m.Rcode = dns.RcodeNameError
				}
			}
		}
	}
}

func handleDnsRequest(w dns.ResponseWriter, r *dns.Msg, store *Store, config *DNSConfig, s *DNSSECSigner) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = false

	switch r.Opcode {
	case dns.OpcodeQuery:
		parseDNSQuery(r, m, store, config, s)
	}

	err := w.WriteMsg(m)
	if err != nil {
		log.Printf("RESPONSE WRITING ERROR: %s", err)
		log.Printf("INvALID RESPONSE:\n%s", m.String())
		return
	}
}

type DNSSECSigner struct {
	d      dns.DNSKEY
	signer crypto.Signer
	config *DNSConfig
	store  *Store
}

func (s *DNSSECSigner) setupRecord() {
	s.d.Hdr = dns.RR_Header{
		Name:   s.store.Domain() + ".",
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
	ds.Hdr.Name = s.store.Domain() + "."
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

func (s *DNSSECSigner) GetSOA() *dns.SOA {
	soa := new(dns.SOA)
	soa.Hdr = dns.RR_Header{
		Name:   s.store.Domain() + ".",
		Rrtype: dns.TypeSOA,
		Class:  dns.ClassINET,
		Ttl:    3600,
	}

	soa.Mbox = s.config.MNAME
	soa.Ns = s.config.NS[0]
	soa.Minttl = 3600
	soa.Refresh = 1
	soa.Retry = 1
	soa.Serial = s.store.GetSerial()
	soa.Expire = 1

	return soa
}

func (s *DNSSECSigner) Sign(rr []dns.RR) (*dns.RRSIG, error) {
	if s.signer == nil {
		return nil, dns.ErrPrivKey
	}

	rrsig := new(dns.RRSIG)
	rrsig.Algorithm = s.d.Algorithm
	rrsig.KeyTag = s.d.KeyTag()
	rrsig.SignerName = s.store.Domain() + "."
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

func ProvideDNS(config *DNSConfig, store *Store, ctx context.Context, errChan chan<- error) {
	// prepare dnssec
	s := &DNSSECSigner{
		config: config,
		store:  store,
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
	mux.HandleFunc(store.Domain()+".", func(w dns.ResponseWriter, r *dns.Msg) {
		handleDnsRequest(w, r, store, config, s)
	})

	// create servers
	serverTcp := &dns.Server{
		Addr:      config.Address + ":" + strconv.Itoa(int(config.Port)),
		Net:       "tcp",
		Handler:   mux,
		ReusePort: true,
	}
	serverUdp := &dns.Server{
		Addr:      config.Address + ":" + strconv.Itoa(int(config.Port)),
		Net:       "udp",
		Handler:   mux,
		UDPSize:   65535,
		ReusePort: true,
	}

	go func() {
		log.Printf("DNS (tcp) listens on %s:%d\n", config.Address, config.Port)
		err := serverTcp.ListenAndServe()
		if err != nil {
			errChan <- err
		}
	}()

	go func() {
		log.Printf("DNS (udp) listens on %s:%d\n", config.Address, config.Port)
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
