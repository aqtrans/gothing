package main

//This file was used by me to reformat the dates on already-uploaded files, from a stupid string-based thing, to a Unix epoch timestamp
//I'm now using a template funcMap to take the Unix timestamp and properly format it when printed
//It takes all keys from the buckets, and sets their timestamp to now

import (
	"encoding/json"
	"github.com/boltdb/bolt"
	"log"
	"time"
)

//BoltDB structs:
type Paste struct {
	Created int64
	Title string
	Content string
	Hits	int64
}

type Snip struct {
	Created int64
	Title string
	Cats string
	Content []string
	Hits	int64
}

type File struct {
	Created int64
	Filename string
	Hits	int64
	RemoteURL string
}

type Image struct {
	Created int64
	Filename string
	Hits	int64
	RemoteURL string
}

type Shorturl struct {
	Created int64
	Short 	string
	Long 	string
	Hits 	int64
}

//OLD
type OPaste struct {
	Created string
	Title string
	Content string
	Hits	int64
}

type OSnip struct {
	Created string
	Title string
	Cats string
	Content []string
	Hits	int64
}

type OFile struct {
	Created string
	Filename string
	Hits	int64
	RemoteURL string
}

type OImage struct {
	Created string
	Filename string
	Hits	int64
	RemoteURL string
}

type OShorturl struct {
	Created string
	Short 	string
	Long 	string
	Hits 	int64
}

func main() {

// Open the database.
Db, _ := bolt.Open("./bolt.db", 0666, nil)
defer Db.Close()

//Lets try this with boltDB now!
Db.Update(func(tx *bolt.Tx) error {
    snips := tx.Bucket([]byte("Snips"))
    snips.ForEach(func(k, v []byte) error {
        var snip *OSnip
        err := json.Unmarshal(v, &snip)
		if err != nil {
			log.Println(err)
		}
        s := &Snip{
            Created: time.Now().Unix(),
            Title: snip.Title,
            Content: snip.Content,
            Hits: snip.Hits,
        }
        encoded, err := json.Marshal(s)
		return snips.Put(k, encoded)
    })
    files := tx.Bucket([]byte("Files"))
    files.ForEach(func(k, v []byte) error {
    	//log.Println("FILES: key="+string(k)+" value="+string(v))
        //fmt.Printf("key=%s, value=%s\n", k, v)
        var file *OFile
        err := json.Unmarshal(v, &file)
		if err != nil {
			log.Println(err)
		}
        f := &File{
            Created: time.Now().Unix(),
            Filename: file.Filename,
            RemoteURL: file.RemoteURL,
            Hits: file.Hits,
        }
        encoded, err := json.Marshal(f)
		return files.Put(k, encoded)
    })
    pastes := tx.Bucket([]byte("Pastes"))
    pastes.ForEach(func(k, v []byte) error {
    	//log.Println("FILES: key="+string(k)+" value="+string(v))
        //fmt.Printf("key=%s, value=%s\n", k, v)
        var paste *OPaste
        err := json.Unmarshal(v, &paste)
		if err != nil {
			log.Println(err)
		}
        p := &Paste{
            Created: time.Now().Unix(),
            Title: paste.Title,
            Content: paste.Content,
            Hits: paste.Hits,
        }
        encoded, err := json.Marshal(p)
		return pastes.Put(k, encoded)
    })
    shorts := tx.Bucket([]byte("Shorturls"))
    shorts.ForEach(func(k, v []byte) error {
    	//log.Println("FILES: key="+string(k)+" value="+string(v))
        //fmt.Printf("key=%s, value=%s\n", k, v)
        var short *OShorturl
        err := json.Unmarshal(v, &short)
		if err != nil {
			log.Println(err)
		}
        sh := &Shorturl{
            Created: time.Now().Unix(),
            Short: short.Short,
            Long: short.Long,
            Hits: short.Hits,
        }
        encoded, err := json.Marshal(sh)
		return shorts.Put(k, encoded)
    })
    images := tx.Bucket([]byte("Images"))
    images.ForEach(func(k, v []byte) error {
    	//log.Println("FILES: key="+string(k)+" value="+string(v))
        //fmt.Printf("key=%s, value=%s\n", k, v)
        var image *OImage
        err := json.Unmarshal(v, &image)
		if err != nil {
			log.Println(err)
		}	
        i := &Image{
            Created: time.Now().Unix(),
            Filename: image.Filename,
            Hits: image.Hits,
            RemoteURL: image.RemoteURL,
        }
        encoded, err := json.Marshal(i)
		return shorts.Put(k, encoded)
    })              
    return nil
})
	
}
