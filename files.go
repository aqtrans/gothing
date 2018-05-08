package main

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/boltdb/bolt"
	raven "github.com/getsentry/raven-go"
)

type File struct {
	Created   int64
	Filename  string
	Hits      int64
	RemoteURL string
}

func (f *File) getType() string {
	return "File"
}

func (f *File) updateHits() {
	log.Println(f.Hits)
	f.Hits = f.Hits + 1
	log.Println(f.Hits)
	err := f.save()
	if err != nil {
		log.Println("Error updateHits:", err)
	}
}

func (f *File) save() error {
	encoded, err := json.Marshal(f)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		return err
	}

	db := getDB()
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Files"))
		return b.Put([]byte(f.Filename), encoded)
	})
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		return err
	}
	log.Println("++++FILE SAVED")
	return nil
}

func (f *File) get(name string) error {
	db := getDB()
	defer db.Close()

	err := db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket([]byte("Files")).Get([]byte(name))
		//Because BoldDB's View() doesn't return an error if there's no key found, just throw a 404 on nil
		//After JSON Unmarshal, Content should be in paste.Content field
		if v == nil {
			return errors.New("Paste does not exist")
		}
		err := json.Unmarshal(v, f)
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
