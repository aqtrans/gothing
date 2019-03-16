package things

type File struct {
	Created   int64
	Filename  string
	Hits      int64
	RemoteURL string
}

func (f *File) GetType() string {
	return "Files"
}

func (f *File) Name() string {
	return f.Filename
}

func (f *File) Date() int64 {
	return f.Created
}

func (f *File) UpdateHits() {
	f.Hits = f.Hits + 1
}
