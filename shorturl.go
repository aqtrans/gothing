package main

import (
	"encoding/json"
	"errors"
	"log"
	"strings"

	"github.com/boltdb/bolt"
	raven "github.com/getsentry/raven-go"
)

type Shorturl struct {
	Created int64
	Short   string
	Long    string
	Hits    int64
}

func (s *Shorturl) getType() string {
	return "Shorturl"
}

func (s *Shorturl) updateHits() {
	log.Println(s.Hits)
	s.Hits = s.Hits + 1
	log.Println(s.Hits)
	err := s.save()
	if err != nil {
		log.Println("Error updateHits:", err)
	}
}

func (s *Shorturl) save() error {
	encoded, err := json.Marshal(s)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		return err
	}

	db := getDB()
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Shorturls"))
		return b.Put([]byte(s.Short), encoded)
	})
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		return err
	}
	log.Println("++++SHORTURL SAVED")
	return nil
}

func (s *Shorturl) get(name string) error {
	shortURL := strings.ToLower(name)
	errNoShortURL := errors.New(shortURL + " - No Such Short URL")

	db := getDB()
	defer db.Close()

	err := db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket([]byte("Shorturls")).Get([]byte(shortURL))
		//Because BoldDB's View() doesn't return an error if there's no key found, just throw a 404 on nil
		//After JSON Unmarshal, Content should be in paste.Content field
		if v == nil {
			return errNoShortURL
		}
		err := json.Unmarshal(v, s)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}
