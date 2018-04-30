package main

import (
	"encoding/json"
	"github.com/boltdb/bolt"
	raven "github.com/getsentry/raven-go"
	"log"
)

type Paste struct {
	*thingDB
	Created int64
	Title   string
	Content string
	Hits    int64
}

func (p *Paste) getType() string {
	return "Paste"
}

func (p *Paste) updateHits() {
	p.Hits = p.Hits + 1
	p.save()
}

func (p *Paste) save() error {
	encoded, err := json.Marshal(p)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		return err
	}

	db := p.getDB()
	defer p.closeDB()

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Pastes"))
		return b.Put([]byte(p.Title), encoded)
	})
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		return err
	}
	log.Println("++++PASTE SAVED")
	return nil
}
