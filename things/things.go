package things

// Thing is the interface that all applicable things should implement
type Thing interface {
	Name() string
	UpdateHits()
	Date() int64
	GetType() string
}

// Sorting functions
type ThingByDate []Thing

func (a ThingByDate) Len() int           { return len(a) }
func (a ThingByDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ThingByDate) Less(i, j int) bool { return a[i].Date() > a[j].Date() }

type ScreenshotByDate []*Screenshot

func (a ScreenshotByDate) Len() int           { return len(a) }
func (a ScreenshotByDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ScreenshotByDate) Less(i, j int) bool { return a[i].Created > a[j].Created }

type ImageByDate []*Image

func (a ImageByDate) Len() int           { return len(a) }
func (a ImageByDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ImageByDate) Less(i, j int) bool { return a[i].Created > a[j].Created }

type PasteByDate []*Paste

func (a PasteByDate) Len() int           { return len(a) }
func (a PasteByDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a PasteByDate) Less(i, j int) bool { return a[i].Created > a[j].Created }

type FileByDate []*File

func (a FileByDate) Len() int           { return len(a) }
func (a FileByDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a FileByDate) Less(i, j int) bool { return a[i].Created > a[j].Created }

type ShortByDate []*Shorturl

func (a ShortByDate) Len() int           { return len(a) }
func (a ShortByDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ShortByDate) Less(i, j int) bool { return a[i].Created > a[j].Created }
