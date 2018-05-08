package main

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/boltdb/bolt"
	raven "github.com/getsentry/raven-go"
)

type Image struct {
	Created   int64
	Filename  string
	Hits      int64
	RemoteURL string
}

func (i *Image) getType() string {
	return "Image"
}

func (i *Image) updateHits() {
	log.Println(i.Hits)
	i.Hits = i.Hits + 1
	log.Println(i.Hits)
	err := i.save()
	if err != nil {
		log.Println("Error updateHits:", err)
	}
}

func (i *Image) save() error {
	encoded, err := json.Marshal(i)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		return err
	}

	db := getDB()
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Images"))
		return b.Put([]byte(i.Filename), encoded)
	})
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		return err
	}
	log.Println("++++IMAGE SAVED")
	return nil
}

func (i *Image) get(name string) error {

	db := getDB()
	defer db.Close()

	err := db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket([]byte("Images")).Get([]byte(name))
		//Because BoldDB's View() doesn't return an error if there's no key found, just throw a 404 on nil
		//After JSON Unmarshal, Content should be in paste.Content field
		if v == nil {
			return errors.New("No such image")
		}
		err := json.Unmarshal(v, i)
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
