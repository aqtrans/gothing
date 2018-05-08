package main

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/boltdb/bolt"
	raven "github.com/getsentry/raven-go"
)

type Screenshot struct {
	Created  int64
	Filename string
	Hits     int64
}

func (s *Screenshot) getType() string {
	return "Image"
}

func (s *Screenshot) updateHits() {
	log.Println(s.Hits)
	s.Hits = s.Hits + 1
	log.Println(s.Hits)
	err := s.save()
	if err != nil {
		log.Println("Error updateHits:", err)
	}
}

func (s *Screenshot) save() error {
	encoded, err := json.Marshal(s)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		return err
	}

	db := getDB()
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Screenshots"))
		return b.Put([]byte(s.Filename), encoded)
	})
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		return err
	}
	log.Println("++++SCREENSHOT SAVED")
	return nil
}

func (s *Screenshot) get(name string) error {

	db := getDB()
	defer db.Close()

	err := db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket([]byte("Screenshots")).Get([]byte(name))
		//Because BoldDB's View() doesn't return an error if there's no key found, just throw a 404 on nil
		//After JSON Unmarshal, Content should be in paste.Content field
		if v == nil {
			return errors.New("No such screenshot")
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
