# give-me-dns

You have IPv6, want to connect to some device and your router can't do mDNS or local domains?

Or, for whatever other reason, you want to be spared the pain of typing out an IPv6 address?

Then give-me-dns(.net) is just right for you!

Simply connect via a TCP client of your liking (like `nc give-me-dns.net 9999`) and you'll get a temporary DNS subdomain

# Development

Start the server: `cd cmd/give-me-dns && go run ../../config.yaml`

Get a name: `nc localhost 9999`

Query the DNS: `dig -p5354 @localhost 1234.give-me-dns.net AAAA`
