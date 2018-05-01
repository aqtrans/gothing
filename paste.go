package main

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/boltdb/bolt"
	raven "github.com/getsentry/raven-go"
)

type Paste struct {
	Created int64
	Title   string
	Content string
	Hits    int64
}

func (p *Paste) getType() string {
	return "Paste"
}

func (p *Paste) updateHits() {
	log.Println(p.Hits)
	p.Hits = p.Hits + 1
	log.Println(p.Hits)
	p.save()
}

func (p *Paste) save() error {
	encoded, err := json.Marshal(p)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		return err
	}

	db := getDB()
	defer db.Close()

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

func getPaste(name string) []byte {
	db := getDB()
	defer db.Close()

	var theBytes []byte

	err := db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket([]byte("Pastes")).Get([]byte(name))
		//Because BoldDB's View() doesn't return an error if there's no key found, just throw a 404 on nil
		//After JSON Unmarshal, Content should be in paste.Content field
		if v == nil {
			return errors.New("Paste does not exist")
		}
		copy(theBytes, v)

		return nil
	})
	if err != nil {
		log.Println(err)
		return nil
	}
	return theBytes
}
