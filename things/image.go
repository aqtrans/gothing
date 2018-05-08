package things

import (
	"log"
)

type Image struct {
	Created   int64
	Filename  string
	Hits      int64
	RemoteURL string
}

func (i *Image) GetType() string {
	return "Images"
}

func (i *Image) Name() string {
	return i.Filename
}

func (i *Image) Date() int64 {
	return i.Created
}

func (i *Image) UpdateHits() {
	log.Println(i.Hits)
	i.Hits = i.Hits + 1
	log.Println(i.Hits)
}
