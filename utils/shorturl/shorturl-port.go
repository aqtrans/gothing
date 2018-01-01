package main

//This file was used by me to change the "schema" of existing ShortURLs
// There is now a FullURL field which needs to be populated

import (
	"encoding/json"
	"github.com/boltdb/bolt"
	"log"
)

type OShorturl struct {
	Created int64
	Short 	string
	Long 	string
	Hits 	int64
}

type Shorturl struct {
	Created int64
	Short   string
    FullURL string
	Long    string
	Hits    int64
}

func main() {

// Open the database.
Db, _ := bolt.Open("./bolt.db", 0666, nil)
defer Db.Close()

//Lets try this with boltDB now!
Db.Update(func(tx *bolt.Tx) error {

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
            Created: short.Created,
            Short: short.Short,
            Long: short.Long,
            Hits: short.Hits,
        }
        encoded, err := json.Marshal(sh)
		return shorts.Put(k, encoded)
    })            
    return nil
})
	
}