package things

import (
	"log"
)

type Paste struct {
	Created int64
	Title   string
	Content string
	Hits    int64
}

func (p *Paste) Name() string {
	return p.Title
}

func (p *Paste) Date() int64 {
	return p.Created
}

func (p *Paste) GetType() string {
	return "Pastes"
}

func (p *Paste) UpdateHits() {
	log.Println(p.Hits)
	p.Hits = p.Hits + 1
	log.Println(p.Hits)
}
