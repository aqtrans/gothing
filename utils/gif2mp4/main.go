package main

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/boltdb/bolt"
)

// This is used to convert existing GIFs to MP4s

type Image struct {
	Created   int64
	Filename  string
	Hits      int64
	RemoteURL string
}

func main() {

	db, err := bolt.Open("thing/bolt.db", 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Fatalln(err)
	}

	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Images"))
		b.ForEach(func(k, v []byte) error {
			log.Println("IMAGE: key=" + string(k) + " value=" + string(v))
			var image *Image
			err := json.Unmarshal(v, &image)
			if err != nil {
				log.Fatalln(err)
			}
			// Convert here:
			if filepath.Ext(image.Filename) == ".gif" {
				log.Println("GIF detected; converting to mp4!", image.Filename)
				nameWithoutExt := image.Filename[0 : len(image.Filename)-len(filepath.Ext(".gif"))]
				path := "./thing/up-imgs/"

				// ffmpeg -i data/up-imgs/filename.gif -vcodec h264 -movflags faststart -y -pix_fmt yuv420p -vf "scale=trunc(iw/2)*2:trunc(ih/2)*2" data/up-imgs/filename.mp4
				resize := exec.Command("/usr/bin/ffmpeg", "-i", filepath.Join(path, image.Filename), "-vcodec", "h264", "-movflags", "faststart", "-y", "-pix_fmt", "yuv420p", "-vf", "scale='trunc(iw/2)*2:trunc(ih/2)*2'", filepath.Join(path, nameWithoutExt+".mp4"))
				err := resize.Run()
				if err != nil {
					log.Fatalln(resize.Args, err)
				}
				// After successful conversion, remove the originally uploaded gif
				err = os.Remove(filepath.Join(path, image.Filename))
				if err != nil {
					log.Fatalln("Error removing gif after converting to mp4", image.Filename, err)
				}
				image.Filename = nameWithoutExt + ".mp4"
				encoded, err := json.Marshal(image)
				if err != nil {
					log.Fatalln(err)
					return err
				}
				return b.Put([]byte(image.Filename), encoded)
			}
			////////////////////
			return nil
		})
		return nil
	})
	if err != nil {
		log.Fatalln(err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Images"))
		b.ForEach(func(k, v []byte) error {
			log.Println("IMAGE: key=" + string(k) + " value=" + string(v))
			var image *Image
			err := json.Unmarshal(v, &image)
			if err != nil {
				log.Fatalln(err)
			}
			// Convert here:
			if filepath.Ext(image.Filename) == ".gif" {
				log.Println("GIF detected; deleting!", image.Filename)
				return b.Delete([]byte(image.Filename))
			}
			////////////////////
			return nil
		})
		return nil
	})
	if err != nil {
		log.Fatalln(err)
	}
}
