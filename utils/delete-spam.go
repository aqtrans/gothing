package delete

//This file was used to delete and re-create the Pastes bucket
// Originally I was going to use this to clean the spam, but legit pastes were too little to warrant the trouble

import (
	//"encoding/json"
	"github.com/boltdb/bolt"
	"log"
	//"time"
)

//BoltDB structs:
type Paste struct {
	Created int64
	Title string
	Content string
	Hits	int64
}


func main() {

// Open the database.
Db, _ := bolt.Open("./data/bolt.db", 0666, nil)
defer Db.Close()

//Lets try this with boltDB now!
Db.Update(func(tx *bolt.Tx) error {
    //pastes := tx.Bucket([]byte("Pastes"))
    err := tx.DeleteBucket([]byte("Pastes"))
    if err != nil {
        log.Println(err)
        return err
    }
    _, err = tx.CreateBucketIfNotExists([]byte("Pastes"))
    if err != nil {
        return err
    }

    pastes := tx.Bucket([]byte("Pastes"))
    pastes.ForEach(func(k, v []byte) error {
        log.Println("Pastes: key="+string(k)+" value="+string(v))
    }

    return nil
})


    	
	
}
