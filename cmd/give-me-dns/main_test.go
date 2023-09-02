package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/miekg/dns"
	"github.com/mkg20001/give-me-dns/lib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"io"
	"net"
	"regexp"
	"testing"
	"time"
)

type GDNSTestSuite struct {
	suite.Suite
	cancel context.CancelFunc
}

func (s *GDNSTestSuite) SetupSuite() {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		err := Init(&lib.Config{
			Domain:    "give-me-dns.net",
			TTL:       48 * time.Hour,
			IDLen:     5,
			DNSPort:   5354,
			DNSMNAME:  "example.example.org",
			DNSNS:     "ns1.give-me-dns.net",
			NetPort:   9999,
			HTTPPort:  8053,
			StoreFile: "/tmp/" + uuid.Must(uuid.NewUUID()).String(),
		}, ctx)
		if err != nil {
			panic(err)
		}
	}()
	s.cancel = cancel
}

var re = regexp.MustCompile(`(?m)Address: ::1\nDNS Name: ([a-z0-9]{5}\.give-me-dns\.net)\nValid for 48h0m0s\nExpires .+\n`)

func (s *GDNSTestSuite) TestEntryAndDNS() {
	time.Sleep(1 * time.Second)

	c, err := net.Dial("tcp", "[::1]:9999")
	if err != nil {
		panic(err)
	}

	res, err := io.ReadAll(c)
	if err != nil {
		panic(err)
	}

	submatch := re.FindSubmatch(res)

	assert.Condition(s.T(), func() (success bool) {
		return submatch != nil
	}, "Message not formatted well")

	address := string(submatch[1]) + "."

	m1 := new(dns.Msg)
	m1.Id = dns.Id()
	m1.Question = make([]dns.Question, 1)
	m1.Question[0] = dns.Question{address, dns.TypeAAAA, dns.ClassINET}

	in, err := dns.Exchange(m1, "[::1]:5354")
	if err != nil {
		panic(err)
	}

	s.Equal(address+"\t172800\tIN\tAAAA\t::1", in.Answer[0].String())

	c2, err := net.Dial("tcp", "[::1]:9999")
	if err != nil {
		panic(err)
	}

	res2, err := io.ReadAll(c2)
	if err != nil {
		panic(err)
	}

	s.Equal(res, res2)
}

func (s *GDNSTestSuite) TearDownSuite() {
	s.cancel()
}

func TestGDNS(t *testing.T) {
	suite.Run(t, &GDNSTestSuite{})
}
