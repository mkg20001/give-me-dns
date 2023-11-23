package idprov

import (
	"embed"
	"math/rand"
	"strings"
)

//go:embed wordlist/words.txt
var assetFS embed.FS

type WordlistID struct {
	Wordlist []string
}

func ProvideWordlistID() (*WordlistID, error) {
	content, err := assetFS.ReadFile("wordlist/words.txt")
	if err != nil {
		return nil, err
	}

	wordlist := strings.Split(string(content), "\n")
	if wordlist[len(wordlist)] == "" {
		wordlist = wordlist[:len(wordlist)-1]
	}

	return &WordlistID{
		Wordlist: wordlist,
	}, nil
}

func (p *WordlistID) GetID() (string, error) {
	return p.Wordlist[rand.Int()%len(p.Wordlist)], nil
}
