package main

// TODO
// - Guard file/image upload pages from respective filetypes
// - Add a screenshot sharing route, separate from image gallery
// - Refactor all save() functions to do the actual file saving as well...
// ...only saving if the BoltDB function doesn't error out


import (
	"crypto/rand"
	"errors"
	"flag"
	"encoding/json"
	"fmt"
	"sort"
	"github.com/zenazn/goji/web"
	"github.com/zenazn/goji/web/middleware"
	"github.com/zenazn/goji/web/mutil"
	"github.com/zenazn/goji/bind"
    "github.com/zenazn/goji/graceful"
	"github.com/hypebeast/gojistatic"
	"github.com/oxtoacart/bpool"
	"github.com/russross/blackfriday"
	"github.com/kennygrant/sanitize"
	"github.com/boltdb/bolt"
	"github.com/prometheus/client_golang/prometheus"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"os/exec"
	"net/http"
	"net/url"
	"runtime"
	"os"
	"time"
	"regexp"
	"strings"
	"strconv"
	"path"
	"bytes"
	"path/filepath"
	"mime"
)

//const timestamp = "2006-01-02_at_03:04:05PM"
const timestamp = "2006-01-02 at 03:04:05PM"

type Configuration struct {
	Port     string
	Username string
	Password string
	Email    string 
	ImgDir   string 
	FileDir  string 
	ThumbDir string
	GifDir   string 
	MainTLD  string 
	ShortTLD string 
	ImageTLD string 
	GifTLD   string
}

var (

    bufpool *bpool.BufferPool
    templates map[string]*template.Template
    _24K int64 = (1 << 20) * 24
	fLocal bool
	Db, _ = bolt.Open("./bolt.db", 0600, nil)
	cfg = Configuration{}

	//Prometheus stuff
    tx_num = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "tkot",
        Subsystem: "tx",
        Name:      "total",
        Help:      "Total number of BoltDB TX requests.",
    })
    tx_page_count = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "tkot",
        Subsystem: "tx",
        Name:      "page_count",
        Help:      "Total number of BoltDB TX pages.",
    })
    tx_cursor_count = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "tkot",
        Subsystem: "tx",
        Name:      "cursor_count",
        Help:      "Total number of BoltDB TX cursors.",
    })
    tx_write_count = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "tkot",
        Subsystem: "tx",
        Name:      "write_count",
        Help:      "Total number of BoltDB TX writes.",
    })       
    tx_write_time = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "tkot",
        Subsystem: "tx",
        Name:      "write_time",
        Help:      "Time spent writing BoltDB transactions.",
    })   
    paste_count = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "tkot",
        Subsystem: "paste",
        Name:      "count",
        Help:      "Total number of Pastes.",
    })
    file_count = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "tkot",
        Subsystem: "file",
        Name:      "count",
        Help:      "Total number of Files.",
    })    
    snips_count = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "tkot",
        Subsystem: "snips",
        Name:      "count",
        Help:      "Total number of Snips.",
    })
    shorturl_count = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "tkot",
        Subsystem: "shorturl",
        Name:      "count",
        Help:      "Total number of Shorturls.",
    })
    images_count = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "tkot",
        Subsystem: "image",
        Name:      "count",
        Help:      "Total number of Images.",
    })
    goroutine_count = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "tkot",
        Name:      "goroutines",
        Help:      "Total number of Goroutines.",
    })      
    memory_allocated = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "tkot",
        Subsystem: "memory",
        Name:      "allocated",
        Help:      "Memory allocated.",
    })  
    memory_mallocs = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "tkot",
        Subsystem: "memory",
        Name:      "mallocs",
        Help:      "Memory mallocs.",
    }) 
    memory_frees = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "tkot",
        Subsystem: "memory",
        Name:      "frees",
        Help:      "Memory frees.",
    })
    memory_gc_total_pause = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "tkot",
        Subsystem: "memory",
        Name:      "gc_total_pause",
        Help:      "Memory GC total pauses.",
    })     
    memory_heap = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "tkot",
        Subsystem: "memory",
        Name:      "heap",
        Help:      "Memory heap size.",
    })     
    memory_stack = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "tkot",
        Subsystem: "memory",
        Name:      "stack",
        Help:      "Memory stack size.",
    })  
    memory_gc_num = prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: "tkot",
        Subsystem: "memory",
        Name:      "gc_num",
        Help:      "Memory GC number.",
    })                       
)

//Flags
//var fLocal = flag.Bool("l", false, "Turn on localhost resolving for Handlers")

//Base struct, Page ; has to be wrapped in a data {} strut for consistency reasons
type Page struct {
	TheName string
    Title   string
    UN      string
    Msg 	string
}

type ListPage struct {
    *Page
    Snips   []*Snip
    Pastes  []*Paste
    Files   []*File
    Shorturls []*Shorturl
    Images  []*Image
}

type GalleryPage struct {
    *Page
    Images  []*Image
}

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

//Sorting functions
type ImageByDate []*Image
func (a ImageByDate) Len() int           { return len(a) }
func (a ImageByDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ImageByDate) Less(i, j int) bool { return a[i].Created < a[j].Created }

type PasteByDate []*Paste
func (a PasteByDate) Len() int           { return len(a) }
func (a PasteByDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a PasteByDate) Less(i, j int) bool { return a[i].Created < a[j].Created }

type SnipByDate []*Snip
func (a SnipByDate) Len() int           { return len(a) }
func (a SnipByDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a SnipByDate) Less(i, j int) bool { return a[i].Created < a[j].Created }

type FileByDate []*File
func (a FileByDate) Len() int           { return len(a) }
func (a FileByDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a FileByDate) Less(i, j int) bool { return a[i].Created < a[j].Created }

type ShortByDate []*Shorturl
func (a ShortByDate) Len() int           { return len(a) }
func (a ShortByDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ShortByDate) Less(i, j int) bool { return a[i].Created < a[j].Created }

func init() {
	//Goji DefaultMux overrides
	bind.WithFlag()
	if fl := log.Flags(); fl&log.Ltime != 0 {
	log.SetFlags(fl | log.Lmicroseconds)
	}
	graceful.DoubleKickWindow(2 * time.Second)

	//Flag '-l' enables go.dev and *.dev domain resolution
	flag.BoolVar(&fLocal, "l", false, "Turn on localhost resolving for Handlers")

	bufpool = bpool.NewBufferPool(64)
	if templates == nil {
		templates = make(map[string]*template.Template)
	}
	templatesDir := "./templates/"
	layouts, err := filepath.Glob(templatesDir + "layouts/*.tmpl")
	if err != nil {
		log.Fatal(err)
	}
	includes, err := filepath.Glob(templatesDir + "includes/*.tmpl")
	if err != nil {
		log.Fatal(err)
	}

    funcMap := template.FuncMap {"prettyDate": PrettyDate, "safeHTML": SafeHTML, "imgClass": ImgClass}

	for _, layout := range layouts {
		files := append(includes, layout)
		//DEBUG TEMPLATE LOADING log.Println(files)
		templates[filepath.Base(layout)] = template.Must(template.New("templates").Funcs(funcMap).ParseFiles(files...))
	}
}

func PrettyDate(date int64) string {
	t := time.Unix(date, 0)
	return t.Format(timestamp)
}

func ImgClass(s string) string {
	if strings.HasSuffix(s, ".gif") {
		return "gifs"
	}
	return "imgs"
}

func SafeHTML(s string) template.HTML {
     return template.HTML(s)
}

func timeTrack(start time.Time, name string) {
    elapsed := time.Since(start)
    log.Printf("[timer] %s took %s", name, elapsed)
}

func markdownRender(content []byte) []byte {
	htmlFlags := 0
	htmlFlags |= blackfriday.HTML_USE_XHTML
	htmlFlags |= blackfriday.HTML_USE_SMARTYPANTS
	htmlFlags |= blackfriday.HTML_SMARTYPANTS_FRACTIONS
	renderer := blackfriday.HtmlRenderer(htmlFlags, "", "")
	extensions := 0
	extensions |= blackfriday.EXTENSION_NO_INTRA_EMPHASIS
	extensions |= blackfriday.EXTENSION_TABLES
	extensions |= blackfriday.EXTENSION_FENCED_CODE
	extensions |= blackfriday.EXTENSION_AUTOLINK
	extensions |= blackfriday.EXTENSION_STRIKETHROUGH
	extensions |= blackfriday.EXTENSION_HARD_LINE_BREAK
	extensions |= blackfriday.EXTENSION_NO_EMPTY_LINE_BEFORE_BLOCK
	extensions |= blackfriday.EXTENSION_AUTO_HEADER_IDS
	return blackfriday.Markdown(content, renderer, extensions)

}

//Hack to allow me to make full URLs due to absence of http:// from URL.Scheme in dev situations
//When behind Nginx, use X-Forwarded-Proto header to retrieve this, then just tack on "://"
//getScheme(r) should return http:// or https://
func getScheme(r *http.Request) (scheme string) {
	scheme = r.Header.Get("X-Forwarded-Proto")+"://"
	/*
	scheme = "http://"
	if r.TLS != nil {
		scheme = "https://"
	}
	*/
	if scheme == "://" {
		scheme = "http://"
	}
	return scheme
}

/*
func GuardPath(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := aaa.Authorize(w, r, true)
		if err != nil {
			fmt.Println(err)
			messages := aaa.Messages(w, r)
			c.Env["msg"] = aaa.Messages(w, r)
			p, err := loadPage("Please log in", "", c)
			data := struct {
	    		Page *Page
			    Title string
			    UN string
			    Msg []string
			} {
	    		p,
	    		"Please log in",
	    		"",
	    		messages,
			}
			err = renderTemplate(w, "login.tmpl", data)
			if err != nil {
			    log.Println(err)
			    return
			}
		}
		user, err := aaa.CurrentUser(w, r)
		if err == nil {
	        if err != nil {
	        	panic(err)
	        }
	        log.Println(user.Username + " is visiting " + r.Referer())
	        next.ServeHTTP(w, r)
		}
	})
}

func GuardAdminPath(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := aaa.AuthorizeRole(w, r, "admin", true)
		if err != nil {
			fmt.Println(err)
			messages := aaa.Messages(w, r)
			c.Env["msg"] = aaa.Messages(w, r)
			p, err := loadPage("Please log in", "", c)
			data := struct {
	    		Page *Page
			    Title string
			    UN string
			    Msg []string
			} {
	    		p,
	    		"Please log in",
	    		"",
	    		messages,
			}
			err = renderTemplate(w, "login.tmpl", data)
			if err != nil {
			    log.Println(err)
			    return
			}

		}
		_, err = aaa.CurrentUser(w, r)
		if err == nil {
	        if err != nil {
	        	panic(err)
	        }
	        next.ServeHTTP(w, r)
		}
	})
}
*/

//func renderTemplate(w http.ResponseWriter, name string, p *Page) error {
func renderTemplate(w http.ResponseWriter, name string, data interface{}) error {
	tmpl, ok := templates[name]
	if !ok {
		return fmt.Errorf("The template %s does not exist", name)
	}

	// Create buffer to write to and check for errors
	buf := bufpool.Get()
	err := tmpl.ExecuteTemplate(buf, "base", data)
	if err != nil {
		bufpool.Put(buf)
		return err
	}

	// Set the header and write the buffer to w
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
	bufpool.Put(buf)
	return nil
}

func indexHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "indexHandler")
	title := "index"
	//c.Env["msg"] = "OMG LOL"
	p, _ := loadMainPage(title, r, c)
	err := renderTemplate(w, "index.tmpl", p)
	if err != nil {
		log.Println(err)
	}
}

func loadGalleryPage(r *http.Request, c web.C) (*GalleryPage, error) {
	defer timeTrack(time.Now(), "loadGalleryPage")
    page, perr := loadPage("Gallery", r, c)
    if perr != nil {
        log.Println(perr)
    }

	var images []*Image
	//Lets try this with boltDB now!
	Db.View(func(tx *bolt.Tx) error {
	    b := tx.Bucket([]byte("Images"))
	    b.ForEach(func(k, v []byte) error {
	        fmt.Printf("key=%s, value=%s\n", k, v)
	        var image *Image
	        err := json.Unmarshal(v, &image)
    		if err != nil {
    			log.Println(err)
    		}
    		images = append(images, image)
    		return nil
	    })
	    return nil
	})
	sort.Sort(ImageByDate(images))
	return &GalleryPage{Page: page, Images: images}, nil
}

func galleryHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "galleryHandler")
	l, err := loadGalleryPage(r, c)
	if err != nil {
		log.Println(err)
	}

	err = renderTemplate(w, "gallery.tmpl", l)
	if err != nil {
		log.Println(err)
	}
}

func galleryEsgyHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "galleryEsgyHandler")
	l, err := loadGalleryPage(r, c)
	if err != nil {
		log.Println(err)
	}

	err = renderTemplate(w, "gallery-esgy.tmpl", l)
	if err != nil {
		log.Println(err)
	}
}

func galleryListHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "galleryListHandler")
	l, err := loadGalleryPage(r, c)
	if err != nil {
		log.Println(err)
	}

	err = renderTemplate(w, "admin-list.tmpl", l)
	if err != nil {
		log.Println(err)
	}
}

func lgHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "lgHandler")
	title := "lg"
	p, err := loadPage(title, r, c)
	data := struct {
		Page *Page
	    Title string
	    Message string
	} {
		p,
		title,
		"",
	}
	err = renderTemplate(w, "lg.tmpl", data)
	if err != nil {
		log.Println(err)
	}
}

func searchHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "searchHandler")
	term := c.URLParams["term"]
	sterm := regexp.MustCompile(term)

	file := &File{}
	paste := &Paste{}
	snip := &Snip{}

	//Lets try this with boltDB now!
	Db.View(func(tx *bolt.Tx) error {
	    b := tx.Bucket([]byte("Snips"))
	    b.ForEach(func(k, v []byte) error {
	        //fmt.Printf("key=%s, value=%s\n", k, v)
	        err := json.Unmarshal(v, &snip)
    		if err != nil {
    			log.Println(err)
    		}
    		slink := snip.Title
    		//sfull := snip.Title + snip.Content
    		if sterm.MatchString(slink) {
    			fmt.Fprintln(w, slink)
    		}
    		for _, scontent := range snip.Content {
	    		if sterm.MatchString(scontent) {
	    			fmt.Fprintln(w, slink)
	    		}
    		}
	        return nil
	    })
	    c := tx.Bucket([]byte("Pastes"))
	    c.ForEach(func(k, v []byte) error {
	        //fmt.Printf("key=%s, value=%s\n", k, v)
	        err := json.Unmarshal(v, &paste)
    		if err != nil {
    			log.Println(err)
    		}
    		plink := paste.Title
    		pfull := paste.Title + paste.Content
    		if sterm.MatchString(pfull) {
    			fmt.Fprintln(w, plink)
    		}
	        return nil
	    })
	    d := tx.Bucket([]byte("Files"))
	    d.ForEach(func(k, v []byte) error {
	        //fmt.Printf("key=%s, value=%s\n", k, v)
	        err := json.Unmarshal(v, &file)
    		if err != nil {
    			log.Println(err)
    		}
    		flink := file.Filename
    		if sterm.MatchString(flink) {
    			fmt.Fprintln(w, flink)
    		}
	        return nil
	    })
	    return nil
	})

}

func uploadPageHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "uploadPageHandler")
	title := "up"
	p, _ := loadMainPage(title, r, c)
	err := renderTemplate(w, "up.tmpl", p)
	if err != nil {
		log.Println(err)
	}
}

func uploadImagePageHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "uploadImagePageHandler")
	title := "upimg"
	p, _ := loadMainPage(title, r, c)
	err := renderTemplate(w, "upimg.tmpl", p)
	if err != nil {
		log.Println(err)
	}
}

func pastePageHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "pastePageHandler")
	title := "paste"
	p, _ := loadMainPage(title, r, c)
	err := renderTemplate(w, "paste.tmpl", p)
	r.ParseForm()
	//log.Println(r.Form)
	if err != nil {
		log.Println(err)
	}
}

func shortenPageHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "shortenPageHandler")
	title := "shorten"
	p, _ := loadMainPage(title, r, c)
	err := renderTemplate(w, "shorten.tmpl", p)
	r.ParseForm()
	//log.Println(r.Form)
	if err != nil {
		log.Println(err)
	}
}

func loginPageHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "loginPageHandler")
	title := "login"
	p, err := loadPage(title, r, c)
	data := struct {
		Page *Page
	    Title string
	} {
		p,
		title,
	}
	err = renderTemplate(w, "login.tmpl", data)
	if err != nil {
	    log.Println(err)
	    return
	}
}


func ParseBool(value string) bool {
    boolValue, err := strconv.ParseBool(value)
    if err != nil {
        return false
    }
    return boolValue
}

func rawSnipHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "rawSnipHandler")
	//vars := mux.Vars(r)
	//title := vars["page"]
	title := c.URLParams["page"]
	snip := &Snip{}
	err := Db.View(func(tx *bolt.Tx) error {
    	v := tx.Bucket([]byte("Snips")).Get([]byte(title))
    	//Because BoldDB's View() doesn't return an error if there's no key found, just throw a 404 on nil
    	//After JSON Unmarshal, Content should be in paste.Content field
    	if v == nil {
			http.Redirect(w, r, "/+edit/"+title, http.StatusFound)
			log.Println("Redirecting to edit page")
			return nil
    	}
    		err := json.Unmarshal(v, &snip)
    		if err != nil {
    			log.Println(err)
    		}
    		//var whole string
    		//for _, val := range snip.Content {
    		//	whole += string(val)
    		//}
    		/*
    		data := struct {
    			Page *Page
    			Snip *Snip
    		} {
    			p,
    			snip,
    		} */
    		//Still using Bluemonday for XSS protection, so some HTML elements can be rendered
    		//Can use template.HTMLEscapeString() if I wanted, which would simply escape stuff
	   		//safe := bluemonday.UGCPolicy().Sanitize(snip.Content)
	   		for s := range snip.Content {
	   			fmt.Fprintln(w, template.HTMLEscapeString(snip.Content[s]))
	   		}
			//fmt.Fprintf(w, "%s", strings.Trim(fmt.Sprint(snip.Content), "[]"))

			//err = renderTemplate(w, "view.tmpl", data)
			//if err != nil {
			//	log.Println(err)
			//}
    	return nil
	})
	if err != nil {
		log.Println(err)
	}
}

func loadPage(title string, r *http.Request, c web.C) (*Page, error) {
	//timer.Step("loadpageFunc")
	m := ""
	if c.Env["msg"] != nil {
		m = c.Env["msg"].(string)	
	}
	user := GetUsername(r, c)
	return &Page{TheName: "GoBanana!", Title: title, UN: user, Msg: m}, nil
}

func loadMainPage(title string, r *http.Request, c web.C) (interface{}, error) {
	//timer.Step("loadpageFunc")
	p, err := loadPage(title, r, c)
	if err != nil {
		return nil, err
	}
	data := struct {
		Page *Page
	} {
		p,
	}
	return data, nil
}

func loadListPage(r *http.Request, c web.C) (*ListPage, error) {
    page, perr := loadPage("List", r, c)
    if perr != nil {
        log.Println(perr)
    }

	var snips []*Snip
	//Lets try this with boltDB now!
	Db.View(func(tx *bolt.Tx) error {
	    b := tx.Bucket([]byte("Snips"))
	    b.ForEach(func(k, v []byte) error {
	    	//log.Println("SNIPS: key="+string(k)+" value="+string(v))
	        //fmt.Printf("key=%s, value=%s\n", k, v)
	        var snip *Snip
	        err := json.Unmarshal(v, &snip)
    		if err != nil {
    			log.Println(err)
    		}
    		snips = append(snips, snip)
	        return nil
	    })
	    return nil
	})
	sort.Sort(SnipByDate(snips))

	var files []*File
	//Lets try this with boltDB now!
	Db.View(func(tx *bolt.Tx) error {
	    b := tx.Bucket([]byte("Files"))
	    b.ForEach(func(k, v []byte) error {
	    	//log.Println("FILES: key="+string(k)+" value="+string(v))
	        //fmt.Printf("key=%s, value=%s\n", k, v)
	        var file *File
	        err := json.Unmarshal(v, &file)
    		if err != nil {
    			log.Println(err)
    		}
    		files = append(files, file)
	        return nil
	    })
	    return nil
	})
	sort.Sort(FileByDate(files))

	/*
	for _, p := range pfiles {
		plink := string(p.Name())
		ptime := p.ModTime().String()
		psize := strconv.FormatInt(p.Size(), 8)
		pl = append(pl, plink)
		pi = append(pi, ptime, psize)
	}
	*/
	var pastes []*Paste
	//Lets try this with boltDB now!
	Db.View(func(tx *bolt.Tx) error {
	    b := tx.Bucket([]byte("Pastes"))
	    b.ForEach(func(k, v []byte) error {
	    	//log.Println("PASTE: key="+string(k)+" value="+string(v))
	        //fmt.Printf("key=%s, value=%s\n", k, v)
	        var paste *Paste
	        err := json.Unmarshal(v, &paste)
    		if err != nil {
    			log.Println(err)
    		}
    		//log.Println(paste)
    		//log.Printf("Addr: %p\n", paste)
    		pastes = append(pastes, paste)
	        return nil
	    })
	    return nil
	})
	sort.Sort(PasteByDate(pastes))
	//log.Println("Pastes: ")
	//log.Println(pastes)
	//log.Println("len:", len(pastes))

	var shorts []*Shorturl
	//Lets try this with boltDB now!
	Db.View(func(tx *bolt.Tx) error {
	    b := tx.Bucket([]byte("Shorturls"))
	    b.ForEach(func(k, v []byte) error {
	    	//log.Println("SHORT: key="+string(k)+" value="+string(v))
	        //fmt.Printf("key=%s, value=%s\n", k, v)
	        var short *Shorturl
	        err := json.Unmarshal(v, &short)
    		if err != nil {
    			log.Println(err)
    		}
    		shorts = append(shorts, short)
	        return nil
	    })
	    return nil
	})
	sort.Sort(ShortByDate(shorts))
	/*
	image := &Image{}
	var images []Image
	//Lets try this with boltDB now!
	Db.View(func(tx *bolt.Tx) error {
	    b := tx.Bucket([]byte("Images"))
	    b.ForEach(func(k, v []byte) error {
	        //fmt.Printf("key=%s, value=%s\n", k, v)
	        err := json.Unmarshal(v, &image)
    		if err != nil {
    			log.Println(err)
    		}
    		ilink := image.Filename
    		//ptime := paste.Created.Format(timestamp)
    		ihits := image.Hits
    		//pl = append(pl, plink)
    		//pi = append(pi, ptime, string(phits))
    		images = []Image{
    			Image{
    			Created: image.Created,
    			Filename: ilink,
    			Hits: ihits,
    			},
    		}
	        img, err := imaging.Open("./up-imgs/"+image.Filename)
	        if err != nil {
	            panic(err)
	        }
	        thumb := imaging.Thumbnail(img, 100, 100, imaging.CatmullRom) 
		    err = imaging.Save(thumb, "./public/thumbs/thumb-"+image.Filename+".jpg")
		    if err != nil {
		        panic(err)
		    }	           		
	        return nil
	    })
	    return nil
	})*/

	var images []*Image
	//Lets try this with boltDB now!
	Db.View(func(tx *bolt.Tx) error {
	    b := tx.Bucket([]byte("Images"))
	    b.ForEach(func(k, v []byte) error {
	        //fmt.Printf("key=%s, value=%s\n", k, v)
	        var image *Image
	        err := json.Unmarshal(v, &image)
    		if err != nil {
    			log.Println(err)
    		}
    		images = append(images, image)
    		return nil
	    })
	    return nil
	})
	sort.Sort(ImageByDate(images))

	return &ListPage{Page: page, Snips: snips, Pastes: pastes, Files: files, Shorturls: shorts, Images: images}, nil
}


func listHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "listHandler")
	l, err := loadListPage(r, c)
	if err != nil {
		log.Println(err)
	}
	err = renderTemplate(w, "list.tmpl", l)
	if err != nil {
		log.Println(err)
	}
}

func APInewRemoteFile(c web.C, w http.ResponseWriter, r *http.Request) {
	remoteURL := r.FormValue("remote")
	finURL := remoteURL
	if !strings.HasPrefix(remoteURL,"http") {
		log.Println("remoteURL does not contain a URL prefix, so adding http")
		finURL = "http://"+remoteURL
	}
	fileURL, err := url.Parse(finURL)
	if err != nil {
		panic(err)
	}	
	path := fileURL.Path
	segments := strings.Split(path, "/")
	fileName := segments[len(segments)-1]
	/*
	log.Println("Filename:")
	log.Println(fileName)
	log.Println("Path:")
	log.Println(path)
	*/
	dlpath := cfg.FileDir
    if r.FormValue("remote-file-name") != "" {
    	log.Println("custom remote file name: "+sanitize.Name(r.FormValue("remote-file-name")))
    	fileName = sanitize.Name(r.FormValue("remote-file-name"))
    }	
	file, err := os.Create(filepath.Join(dlpath, fileName))
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	defer file.Close()
	check := http.Client{
			CheckRedirect: func(r *http.Request, via [] *http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
			},
	}
	resp, err := check.Get(finURL)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	defer resp.Body.Close()
	fmt.Println(resp.Status)

	size, err := io.Copy(file, resp.Body)
	if err != nil {
		panic(err)
	}

	//BoltDB stuff
    fi := &File{
        Created: time.Now().Unix(),
        Filename: fileName,
        RemoteURL: finURL,
    }
    err = fi.save()
    if err != nil {
        log.Println(err)
    }

	//fmt.Printf("%s with %v bytes downloaded", fileName, size)
	//fmt.Fprintf(w, "%s with %v bytes downloaded from %s", fileName, size, finURL)
	fmt.Printf("%s with %v bytes downloaded from %s", fileName, size, finURL)
	//log.Println("Filename:")
	//log.Println(fileName)

	c.Env["msg"] = fileName+" successfully uploaded! | <a style='color:#fff' href=/d/"+fileName+"><i class='fa fa-link'></i>Link</a>"
	title := fileName+" successfully uploaded!"
	p, _ := loadMainPage(title, r, c)
	w.Header().Set("Location", "http://go.jba.io/up")
	w.WriteHeader(303)
	err = renderTemplate(w, "up.tmpl", p)
	if err != nil {
		log.Println(err)
	}

}

func ParseMultipartFormProg(r *http.Request, maxMemory int64) error {
	//length := r.ContentLength
	//ticker := time.Tick(time.Millisecond)

	if r.Form == nil {
		err := r.ParseForm()
		if err != nil {
			return err
		}
	}
	if r.MultipartForm != nil {
		return nil
	}

	mr, err := r.MultipartReader()
	if err != nil {
		return err
	}

	f, err := mr.ReadForm(maxMemory)
	if err != nil {
		return err
	}
	for k, v := range f.Value {
		r.Form[k] = append(r.Form[k], v...)
	}
	r.MultipartForm = f

	return nil
}

func APInewFile(c web.C, w http.ResponseWriter, r *http.Request) {
	//vars := mux.Vars(r)
	contentLength := r.ContentLength
	var reader io.Reader
	var f io.WriteCloser
	var err error
	var filename string
	//var cli bool
	//var remote bool
	var uptype string
	var fi *File
	//fi := &File{}
	path := cfg.FileDir
	contentType := r.Header.Get("Content-Type")

	//Determine how the file is being uploaded
	if r.FormValue("remote") != "" {
		uptype = "remote"
	} else if contentType == "" {
		uptype = "cli"
	} else {
		uptype = "form"
	}
	//log.Println(uptype)

	//Remote File Uploads
	if uptype == "remote" {
		remoteURL := r.FormValue("remote")
		finURL := remoteURL
		if !strings.HasPrefix(remoteURL,"http") {
			log.Println("remoteURL does not contain a URL prefix, so adding http")
			finURL = "http://"+remoteURL
		}
		fileURL, err := url.Parse(finURL)
		if err != nil {
			panic(err)
		}	
		//path := fileURL.Path
		segments := strings.Split(fileURL.Path, "/")
		filename = segments[len(segments)-1]
		/*
		log.Println("Filename:")
		log.Println(fileName)
		log.Println("Path:")
		log.Println(path)
		*/
		//dlpath := cfg.FileDir
	    if r.FormValue("remote-file-name") != "" {
	    	log.Println("custom remote file name: "+sanitize.Name(r.FormValue("remote-file-name")))
	    	filename = sanitize.Name(r.FormValue("remote-file-name"))
	    }
		file, err := os.Create(filepath.Join(path, filename))
		if err != nil {
			fmt.Println(err)
			panic(err)
		}
		defer file.Close()
		check := http.Client{
				CheckRedirect: func(r *http.Request, via [] *http.Request) error {
				r.URL.Opaque = r.URL.Path
				return nil
				},
		}
		resp, err := check.Get(finURL)
		if err != nil {
			fmt.Println(err)
			panic(err)
		}
		defer resp.Body.Close()
		fmt.Println(resp.Status)

		size, err := io.Copy(file, resp.Body)
		if err != nil {
			panic(err)
		}

		//BoltDB stuff
	    fi = &File{
	        Created: time.Now().Unix(),
	        Filename: filename,
	        RemoteURL: finURL,
	    }

		//fmt.Printf("%s with %v bytes downloaded", fileName, size)
		//fmt.Fprintf(w, "%s with %v bytes downloaded from %s", fileName, size, finURL)
		fmt.Printf("%s with %v bytes downloaded from %s", filename, size, finURL)
	} else if uptype == "cli" {
		log.Println("Content-type blank, so this should be a CLI upload...")
		//Then this should be an upload from the command line...
		reader = r.Body
		if contentLength == -1 {
			var err error
			var f io.Reader
			f = reader
			var b bytes.Buffer
			n, err := io.CopyN(&b, f, _24K+1)
			if err != nil && err != io.EOF {
				log.Printf("%s", err.Error())
				http.Error(w, err.Error(), 500)
				return
			}
			if n > _24K {
				file, err := ioutil.TempFile("./tmp/", "transfer-")
				if err != nil {
					log.Printf("%s", err.Error())
					http.Error(w, err.Error(), 500)
					return
				}
				defer file.Close()
				n, err = io.Copy(file, io.MultiReader(&b, f))
				if err != nil {
					os.Remove(file.Name())
					log.Printf("%s", err.Error())
					http.Error(w, err.Error(), 500)
					return
				}
				reader, err = os.Open(file.Name())
			} else {
				reader = bytes.NewReader(b.Bytes())
			}
			contentLength = n
		}
		filename = sanitize.Path(filepath.Base(c.URLParams["id"]))
		log.Println(filename)
		if filename == "." {
			log.Println("Filename is blank " + filename)
			dictionary := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
			var bytes = make([]byte, 4)
			rand.Read(bytes)
			for k, v := range bytes {
				bytes[k] = dictionary[v%byte(len(dictionary))]
			}
			filename = string(bytes)
		}
	    if r.FormValue("local-file-name") != "" {
	    	log.Println("custom local file name: "+sanitize.Name(r.FormValue("local-file-name")))
	    	filename = sanitize.Name(r.FormValue("local-file-name"))
	    }		
		log.Printf("Uploading %s %d %s", filename, contentLength, contentType)
		
		if f, err = os.OpenFile(filepath.Join(path, filename), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600); err != nil {
			fmt.Printf("%s", err.Error())
			http.Error(w, errors.New("Could not save file").Error(), 500)
			return
		}
		defer f.Close()
		if _, err = io.Copy(f, reader); err != nil {
			return
		}		
		contentType = mime.TypeByExtension(filepath.Ext(c.URLParams["id"]))
		//BoltDB stuff
	    fi = &File{
	        Created: time.Now().Unix(),
	        Filename: filename,
	    }			
	} else if uptype == "form" {
        //log.Println("Content-type is "+contentType)
        err := r.ParseMultipartForm(_24K)
        if err != nil {
            log.Println("ParseMultiform reader error")
            log.Println(err)
            return        	
        }
        file, handler, err := r.FormFile("file")
		filename = handler.Filename
		defer file.Close()
		if err != nil {
			fmt.Println(err)
		}
        if r.FormValue("local-file-name") != "" {
        	log.Println("CUSTOM FILENAME: ")
        	log.Println(r.FormValue("local-file-name"))
        	filename = r.FormValue("local-file-name")
        }

        f, err := os.OpenFile(filepath.Join(path, filename), os.O_WRONLY|os.O_CREATE, 0666)
        if err != nil {
            fmt.Println(err)
            return
        }
        defer f.Close()
        io.Copy(f, file)

		//BoltDB stuff
	    fi = &File{
	        Created: time.Now().Unix(),
	        Filename: filename,
	    }		
	}	

    err = fi.save()
    if err != nil {
        log.Println(err)
    }

    if uptype == "cli" {
    	fmt.Fprintf(w, "http://go.jba.io/d/"+filename)
    } else {
		c.Env["msg"] = filename+" successfully uploaded! | <a style='color:#fff' href=/d/"+filename+"><i class='fa fa-link'></i>Link</a>"
		title := filename+" successfully uploaded!"
		p, _ := loadMainPage(title, r, c)
		err = renderTemplate(w, "up.tmpl", p)
		if err != nil {
			log.Println(err)
		}    	
    }
}

func (f *File) save() error {
    err := Db.Update(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte("Files"))
        encoded, err := json.Marshal(f)
        if err != nil {
        	log.Println(err)
            return err
        }
        return b.Put([]byte(f.Filename), encoded)
    })
    if err != nil {
    	log.Println(err)
    	return err
    }
	//log.Println(f)    
    log.Println("++++FILE SAVED")
    return nil
}



//Short URL Handlers
func shortUrlHandler(w http.ResponseWriter, r *http.Request) {

	defer timeTrack(time.Now(), "shortUrlHandler")

	shorturl := &Shorturl{}

	//The Host that the user queried.
	host := r.Host
	host = strings.TrimSpace(host)
	//Figure out if a subdomain exists in the host given.
	host_parts := strings.Split(host, ".")
	subdomain := ""
	log.Println("Received Short URL request for "+host)
	if len(host_parts) > 2 {
	    //The subdomain exists, we store it as the first element 
	    //in a new array
	    subdomain = string(host_parts[0])
	}
	title := subdomain
	err := Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Shorturls"))
    	v := b.Get([]byte(title))
    	//Because BoldDB's View() doesn't return an error if there's no key found, just throw a 404 on nil
    	//After JSON Unmarshal, Content should be in paste.Content field
    	if v == nil {
			http.Error(w, "Error 400 - No such domain at this address", 400)
			err := errors.New(title + "No Such Short URL")
			return err
			//log.Println(err)
    	} else {
    		err := json.Unmarshal(v, &shorturl)
    		if err != nil {
    			log.Println(err)
    		}
	        count := (shorturl.Hits + 1)
	        //If the shorturl is local, just serve whatever file being requested
	        if strings.Contains(shorturl.Long, cfg.ShortTLD+"/") {
	        	log.Println("LONG URL CONTAINS ShortTLD")
	        	if strings.HasPrefix(shorturl.Long, "http://"+cfg.ImageTLD) {
	        		u, err := url.Parse(shorturl.Long)
	        		if err != nil {
	        			log.Println(err)
	        		}
				    segments := strings.Split(u.Path, "/")
				    fileName := segments[len(segments)-1]	        		
	        		log.Println("Serving "+shorturl.Long+" file directly")
	        		http.ServeFile(w, r, cfg.ImgDir+fileName) 
	        	}
	        }
	        if strings.Contains(shorturl.Long, cfg.MainTLD+"/i/") {
	        	log.Println("LONG URL CONTAINS MainTLD")
	        	if strings.HasPrefix(shorturl.Long, "http://"+cfg.MainTLD+"/i/") {
	        		u, err := url.Parse(shorturl.Long)	        		
	        		if err != nil {
	        			log.Println(err)
	        		}
				    segments := strings.Split(u.Path, "/")
				    fileName := segments[len(segments)-1]	        		
	        		log.Println("Serving "+shorturl.Long+" file directly")
	        		http.ServeFile(w, r, cfg.ImgDir+fileName) 
	        	}
	        }	        
	        http.Redirect(w, r, shorturl.Long, 302)

	        s := &Shorturl{
	            Created: shorturl.Created,
	            Short: shorturl.Short,
	            Long: shorturl.Long,
	            Hits: count,
	        }
	        encoded, err := json.Marshal(s)
	        /*
    		data := struct {
    			Page *Page
    			Snip *Snip
    		} {
    			p,
    			s,
    		}
    		//Still using Bluemonday for XSS protection, so some HTML elements can be rendered
    		//Can use template.HTMLEscapeString() if I wanted, which would simply escape stuff
	   		//safe := bluemonday.UGCPolicy().Sanitize(snip.Content)
			//fmt.Fprintf(w, "%s", data)
			err = renderTemplate(w, "view.tmpl", data)
			if err != nil {
				log.Println(err)
			}*/

		//return nil
    	return b.Put([]byte(title), encoded)
    	}
	})
	if err != nil {
		log.Println(err)
	}
}

func APInewShortUrlForm(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "APInewShortUrlForm")
	//vars := mux.Vars(r)
	//var name = ""
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
	}
	short := r.PostFormValue("short")
	if short != "" {
		short = short
	} else {
		dictionary := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
		var bytes = make([]byte, 4)
		rand.Read(bytes)
		for k, v := range bytes {
			bytes[k] = dictionary[v%byte(len(dictionary))]
		}
		short = string(bytes)
	}
	long := r.PostFormValue("long")
	s := &Shorturl{
	    Created: time.Now().Unix(),
	    Short: short,
	    Long: long,
	}

	/*
	Created string
	Short 	string
	Long 	string
	*/

	err = s.save()
	if err != nil {
		log.Println(err)
	}
	//http.Redirect(w, r, myURL + "/p/" + title, 302)
    //fmt.Fprintln(w, "Your Short URL is available at: %s", s.Short)
	log.Println("Short: " + s.Short)
	log.Println("Long: " + s.Long)

	c.Env["msg"] = "Your short URL is available at: <a style='color:#fff' href='http://"+s.Short+".es.gy/'><i class='fa fa-link'></i>"+s.Short+"</a>"
	title := "New ShortURL available"
	p, _ := loadMainPage(title, r, c)
	w.Header().Set("Location", "http://go.jba.io/s")
	w.WriteHeader(303)
	err = renderTemplate(w, "shorten.tmpl", p)
	if err != nil {
		log.Println(err)
	}

}

func (s *Shorturl) save() error {
	err := Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Shorturls"))
	    encoded, err := json.Marshal(s)
	    if err != nil {
	    	return err
	    }
	    return b.Put([]byte(s.Short), encoded)
	})
    if err != nil {
    	return err
    }	
	log.Println("++++SHORTURL SAVED")
	return nil
}

//Pastebin handlers
func APInewPaste(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "APInewPaste")
	log.Println("Paste request...")
	paste := r.Body
	buf := new(bytes.Buffer)
	buf.ReadFrom(paste)
	bpaste := buf.String()
	var name = ""
	if c.URLParams["id"] != "" {
		name = c.URLParams["id"]
	} else {
		dictionary := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
		var bytes = make([]byte, 4)
		rand.Read(bytes)
		for k, v := range bytes {
			bytes[k] = dictionary[v%byte(len(dictionary))]
		}
		name = string(bytes)
	}
	p := &Paste{
	    Created: time.Now().Unix(),
	    Title: name,
	    Content: bpaste,
	}
	err := p.save()
	if err != nil {
		log.Println(err)
	}
	fmt.Fprintln(w, getScheme(r)+r.Host+"/p/"+name)
}

func APInewPasteForm(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "APInewPasteForm")
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
	}
	title := r.PostFormValue("title")
	if title != "" {
		title = title
	} else {
		dictionary := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
		var bytes = make([]byte, 4)
		rand.Read(bytes)
		for k, v := range bytes {
			bytes[k] = dictionary[v%byte(len(dictionary))]
		}
		title = string(bytes)
	}
	paste := r.PostFormValue("paste")
	p := &Paste{
	    Created: time.Now().Unix(),
	    Title: title,
	    Content: paste,
	}
	err = p.save()
	if err != nil {
		log.Println(err)
	}
	http.Redirect(w, r, getScheme(r)+r.Host+"/p/"+title, 302)
}

func (p *Paste) save() error {
	err := Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Pastes"))
	    encoded, err := json.Marshal(p)
	    if err != nil {
	    	log.Println(err)
	    	return err
	    }
	    return b.Put([]byte(p.Title), encoded)
	})
    if err != nil {
    	log.Println(err)
    	return err
    }	
	log.Println("++++PASTE SAVED")
	return nil
}

func pasteHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "pasteHandler")
	title := c.URLParams["id"]
	paste := &Paste{}
	err := Db.View(func(tx *bolt.Tx) error {
    	v := tx.Bucket([]byte("Pastes")).Get([]byte(title))
    	//Because BoldDB's View() doesn't return an error if there's no key found, just throw a 404 on nil
    	//After JSON Unmarshal, Content should be in paste.Content field
    	if v == nil {
    		http.NotFound(w, r)
    		return nil
    	}
    		err := json.Unmarshal(v, &paste)
    		if err != nil {
    			log.Println(err)
    		}
    		//No longer using BlueMonday or template.HTMLEscapeString because theyre too overzealous
    		//I need '<' and '>' in tact for scripts and such

	   		//safe := template.HTMLEscapeString(paste.Content)
	   		//safe := sanitize.HTML(paste.Content)

	   		safe := strings.Replace(paste.Content, "<script>", "< script >", -1)
	   		//safe := paste.Content
			fmt.Fprintf(w, "%s", safe)
    		return nil
	})
	if err != nil {
		log.Println(err)
	}

    //Attempt to increment paste hit counter...
    err = Db.Update(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte("Pastes"))
        v := b.Get([]byte(title))
        //If there is no existing key, do not do a thing
        if v == nil {
        	http.NotFound(w, r)
        	return nil
        }
        err := json.Unmarshal(v, &paste)
        if err != nil {
            log.Println(err)
        }
        count := (paste.Hits + 1)
        p := &Paste{
            Created: paste.Created,
            Title: paste.Title,
            Content: paste.Content,
            Hits: count,
        }
        encoded, err := json.Marshal(p)
        return b.Put([]byte(title), encoded)
    })
    if err != nil{
    	log.Println(err)
    }
}

//Snip handlers
func editSnipHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "editSnipHandler")
	title := c.URLParams["page"]
	snip := &Snip{}
	p, err := loadPage(title, r, c)
	if err != nil {
		log.Println(err)
	}
	err = Db.View(func(tx *bolt.Tx) error {
    	v := tx.Bucket([]byte("Snips")).Get([]byte(title))
    	//Because BoldDB's View() doesn't return an error if there's no key found, just render an empty page to edit
    	//After JSON Unmarshal, Content should be in paste.Content field
    	if v == nil {
			p = &Page{Title: title}
			s := &Snip{Created: time.Now().Unix(), Title: title,}
			data := struct {
				Page *Page
				Snip *Snip
			} {
				p,
				s,
			}
			err = renderTemplate(w, "edit.tmpl", data)
			if err != nil {
				log.Println(err)
			}
			return nil
			//log.Println(err)
    	}
    		err := json.Unmarshal(v, &snip)
    		if err != nil {
    			log.Println(err)
    		}
    		var whole string
    		for _, val := range snip.Content {
    			whole += string(val)
    		}
    		data := struct {
    			Page *Page
    			Snip *Snip
    			Content string
    		} {
    			p,
    			snip,
    			whole,
    		}
    		//Still using Bluemonday for XSS protection, so some HTML elements can be rendered
    		//Can use template.HTMLEscapeString() if I wanted, which would simply escape stuff
	   		//safe := bluemonday.UGCPolicy().Sanitize(snip.Content)
			//fmt.Fprintf(w, "%s", snip.Content)
			err = renderTemplate(w, "edit.tmpl", data)
			if err != nil {
				log.Println(err)
			}
    		return nil
	})
	if err != nil {
		log.Println(err)
	}


}

func snipHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "snipHandler")
	//title := vars["page"]
	title := c.URLParams["page"]
	snip := &Snip{}
	p, err := loadPage(title, r, c)
//	err = Db.View(func(tx *bolt.Tx) error {
	err = Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Snips"))
    	v := b.Get([]byte(title))
    	//Because BoldDB's View() doesn't return an error if there's no key found, just throw a 404 on nil
    	//After JSON Unmarshal, Content should be in paste.Content field
    	if v == nil {
			http.Redirect(w, r, "/+edit/"+title, http.StatusFound)
			return nil
    	}
    		err := json.Unmarshal(v, &snip)
    		if err != nil {
    			log.Println(err)
    		}
	        count := (snip.Hits + 1)
	        s := &Snip{
	            Created: snip.Created,
	            Title: snip.Title,
	            Content: snip.Content,
	            Hits: count,
	        }
	        encoded, err := json.Marshal(s)

    		data := struct {
    			Page *Page
    			Snip *Snip
    		} {
    			p,
    			s,
    		}
    		//Still using Bluemonday for XSS protection, so some HTML elements can be rendered
    		//Can use template.HTMLEscapeString() if I wanted, which would simply escape stuff
	   		//safe := bluemonday.UGCPolicy().Sanitize(snip.Content)
			//fmt.Fprintf(w, "%s", data)
			err = renderTemplate(w, "view.tmpl", data)
			if err != nil {
				log.Println(err)
			}
		//return nil
    	return b.Put([]byte(title), encoded)
	})
	if err != nil {
		log.Println(err)
	}
	/*
    //Attempt to increment paste hit counter...
    err = Db.Update(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte("Snips"))
        v := b.Get([]byte("snip-"+title))
        err := json.Unmarshal(v, &snip)
        if err != nil {
            log.Println(err)
        }
        count := (snip.Hits + 1)
        s := &Snip{
            Created: snip.Created,
            Title: snip.Title,
            Content: snip.Content,
            Hits: count,
        }
        encoded, err := json.Marshal(s)
        return b.Put([]byte("snip-"+title), encoded)
    })
    if err != nil{
    	log.Println(err)
    } */

}

func APIsaveSnip(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "APIsaveSnip")
	title := c.URLParams["page"]
	body := r.FormValue("body")
	fmattercats := r.FormValue("fmatter-cats")
	//newbody := strings.Replace(body, "\r", "", -1)
	bodslice := []string{}
	bodslice = append(bodslice, body)
	s := &Snip{
	    Created: time.Now().Unix(),
	    Title: title,
	    Cats: fmattercats,
	    Content: bodslice,
	}
	err := s.save()
	if err != nil {
		log.Println(err)
	}
	http.Redirect(w, r, "/"+title, http.StatusFound)
	log.Println(title + " page saved!")
}

func APIappendSnip(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "APIappendSnip")
	title := c.URLParams["page"]
	body := r.FormValue("append")
	snip := &Snip{}
	err := Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Snips"))
		v := b.Get([]byte(title))
		err := json.Unmarshal(v, &snip)
		if err != nil {
			log.Println(err)
		}
		newslice := snip.Content
		newslice = append(newslice, body)
		s := &Snip {
			Title: title,
			Content: newslice,
		}
	    encoded, err := json.Marshal(s)
	    if err != nil {
	    	return err
	    }
		log.Println("++++SNIP APPENDED")
	    return b.Put([]byte(title), encoded)
	})
	if err != nil {
		log.Println(err)
	}
	http.Redirect(w, r, "/"+title, http.StatusFound)
	log.Println(title + " page saved!")
}

func (s *Snip) save() error {
	err := Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Snips"))
	    encoded, err := json.Marshal(s)
	    if err != nil {
	    	log.Println(err)
	    	return err
	    }
		log.Println("++++SNIP SAVED")
		log.Println(string(encoded))
	    return b.Put([]byte(s.Title), encoded)
	})
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func downloadHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "downloadHandler")
    name := c.URLParams["name"]
    fpath := cfg.FileDir + path.Base(name)

    //Attempt to increment file hit counter...
    file := &File{}
    Db.Update(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte("Files"))
        v := b.Get([]byte(name))
        //If there is no existing key, do not do a thing
        if v == nil {
        	http.NotFound(w, r)
        	return nil
        }
        err := json.Unmarshal(v, &file)
        if err != nil {
            log.Println(err)
        }
        count := (file.Hits + 1)
        fi := &File{
            Created: file.Created,
            Filename: file.Filename,
            Hits: count,
        }
        encoded, err := json.Marshal(fi)
        http.ServeFile(w, r, fpath)
        return b.Put([]byte(name), encoded)
    })
    
}

func downloadImageHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "downloadImageHandler")
    name := c.URLParams["name"]
    fpath := cfg.ImgDir + path.Base(name)

    //Attempt to increment file hit counter...
    image := &Image{}
    Db.Update(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte("Images"))
        v := b.Get([]byte(name))
        //If there is no existing key, do not do a thing
        if v == nil {
        	http.NotFound(w, r)
        	return nil
        }
        err := json.Unmarshal(v, &image)
        if err != nil {
            log.Println(err)
        }
        count := (image.Hits + 1)
        imi := &Image{
            Created: image.Created,
            Filename: image.Filename,
            Hits: count,
        }
        encoded, err := json.Marshal(imi)
        return b.Put([]byte(name), encoded)
    })
    http.ServeFile(w, r, fpath)
}

//Separate function so thumbnail displays on the Gallery page do not increase hit counter
//TODO: Probably come up with a better way to do this, IP based exclusion perhaps?
func imageThumbHandler(c web.C, w http.ResponseWriter, r *http.Request) {
    name := c.URLParams["name"]
    fpath := cfg.ImgDir + path.Base(strings.TrimSuffix(name, ".png"))
    log.Println("name:"+ name)
    log.Println("fpath:"+ fpath)
//    http.ServeFile(w, r, fpath)

    thumbPath := cfg.ThumbDir+path.Base(name)
    log.Println("thumbpath:"+thumbPath)

    //Check to see if the large image already exists
    //If so, serve it directly
	if _, err := os.Stat(thumbPath); err == nil {
		log.Println("Pre-existing thumbnail already found, serving it...")
		http.ServeFile(w, r, cfg.ThumbDir+path.Base(name))
	} else {
		log.Println("Thumbnail not found. Running imagemagick...")
		file, err := os.Open(fpath)
		if err != nil {
		     log.Println(err)
		     return
		}
		file.Close()
		//gifsicle --conserve-memory --colors 256 --resize 2000x_ ./up-imgs/groove_fox.gif -o ./tmp/BIG-groove_fox.gif
		//convert -define "jpeg:size=300x300 -thumbnail 300x300 ./up-imgs/

		resize := exec.Command("/usr/bin/convert", fpath, "-strip", "-thumbnail","x300", thumbPath)
    	contentType := mime.TypeByExtension(filepath.Ext(path.Base(strings.TrimSuffix(name, ".png"))))
    	if contentType == "image/gif" {
    		gpath := fpath+"[0]"
			resize = exec.Command("/usr/bin/convert", gpath, "-strip", "-thumbnail","x300", thumbPath)
		}
		//resize := exec.Command("/usr/bin/gifsicle", "--conserve-memory", "--resize-height", "300", fpath, "#0", "-o", thumbPath)
		err = resize.Run()
		if err != nil {
			log.Println(err)
		}
	    http.ServeFile(w, r, cfg.ThumbDir+path.Base(name))
	}

}

func imageDirectHandler(c web.C, w http.ResponseWriter, r *http.Request) {
    name := c.URLParams["name"]
    fpath := cfg.ImgDir + path.Base(name)
    http.ServeFile(w, r, fpath)
}


//Resizes all images using gifsicle command, due to image.resize failing at animated GIFs
//Images are dumped to ./tmp/ for now, probably want to fix this but I'm unsure where to put them
func imageBigHandler(c web.C, w http.ResponseWriter, r *http.Request) {
    name := c.URLParams["name"]
    smallPath := cfg.ImgDir+path.Base(name)
    bigPath := cfg.GifDir+path.Base(name)

    //Check to see if the large image already exists
    //If so, serve it directly
	if _, err := os.Stat(bigPath); err == nil {
		log.Println("Pre-existing BIG gif already found, serving it...")
		http.ServeFile(w, r, cfg.GifDir+path.Base(name))
	} else {
		log.Println("BIG gif not found. Running gifsicle...")
		file, err := os.Open(smallPath)
		if err != nil {
		     log.Println(err)
		     return
		}
		file.Close()
		//gifsicle --conserve-memory --colors 256 --resize 2000x_ ./up-imgs/groove_fox.gif -o ./tmp/BIG-groove_fox.gif
		resize := exec.Command("/usr/bin/gifsicle", "--conserve-memory", "--colors", "256","--resize", "2000x_", smallPath, "-o", bigPath)
		err = resize.Run()
		if err != nil {
			log.Println(err)
		}
	    http.ServeFile(w, r, cfg.GifDir+name)
	}
}

//Separate function to resize GIFs in a goroutine
func embiggenHandler(i string) {
    name := i
    smallPath := cfg.ImgDir+path.Base(name)
    bigPath := cfg.GifDir+path.Base(name)

    //Check to see if the large image already exists
    //If so, serve it directly
	if _, err := os.Stat(bigPath); err == nil {
		log.Println("Pre-existing BIG gif already found, serving it...")
		return
	} else {
		log.Println("BIG gif not found. Running gifsicle...")
		file, err := os.Open(smallPath)
		if err != nil {
		     log.Println(err)
		     return
		}
		file.Close()
		//gifsicle --conserve-memory --colors 256 --resize 2000x_ ./up-imgs/groove_fox.gif -o ./tmp/BIG-groove_fox.gif
		resize := exec.Command("/usr/bin/gifsicle", "--conserve-memory", "--colors", "256","--resize", "2000x_", smallPath, "-o", bigPath)
		err = resize.Run()
		if err != nil {
			log.Println(err)
		}
	    log.Println(name+" BIG GIF has been saved.")
	}
}

//Delete stuff
//TODO: Add images to this
func APIdeleteHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	//Requests should come in on /api/delete/{type}/{name}
	ftype := c.URLParams["type"]
	fname := c.URLParams["name"]
	if ftype == "snip" {
		err := Db.Update(func(tx *bolt.Tx) error {
			log.Println(ftype + " " + fname + " has been deleted")		
		    return tx.Bucket([]byte("Snips")).Delete([]byte(fname))
		})
		if err != nil {
			log.Println(err)
			return
		}
		c.Env["msg"] = "Snip " + fname + " has been deleted"
		http.Redirect(w, r, "/list", http.StatusSeeOther)
	} else if ftype == "file" {
		err := Db.Update(func(tx *bolt.Tx) error {
			log.Println(ftype + " " + fname + " has been deleted")
		    return tx.Bucket([]byte("Files")).Delete([]byte(fname))
		})
		if err != nil {
			log.Println(err)
			return
		}
		fpath := cfg.FileDir + fname
		err = os.Remove(fpath)
		if err != nil {
			log.Println(err)
			return
		}
		c.Env["msg"] = "File " + fname + " has been deleted"
		http.Redirect(w, r, "/list", http.StatusSeeOther)
	} else if ftype == "image" {
		err := Db.Update(func(tx *bolt.Tx) error {
			log.Println(ftype + " " + fname + " has been deleted")
		    return tx.Bucket([]byte("Images")).Delete([]byte(fname))
		})
		if err != nil {
			log.Println(err)
			return
		}
		fpath := cfg.ImgDir + fname
		err = os.Remove(fpath)
		if err != nil {
			log.Println(err)
			return
		}
		c.Env["msg"] = "Image " + fname + " has been deleted"
		http.Redirect(w, r, "/list", http.StatusSeeOther)
	} else if ftype == "paste" {
		err := Db.Update(func(tx *bolt.Tx) error {
			log.Println(ftype + " " + fname + " has been deleted")
		    return tx.Bucket([]byte("Pastes")).Delete([]byte(fname))
		})
		if err != nil {
			log.Println(err)
		}
		c.Env["msg"] = "Paste " + fname + " has been deleted"
		http.Redirect(w, r, "/list", http.StatusSeeOther)
	} else if ftype == "shorturl" {
		err := Db.Update(func(tx *bolt.Tx) error {
			log.Println(ftype + " " + fname + " has been deleted")
		    return tx.Bucket([]byte("Shorturls")).Delete([]byte(fname))
		})
		if err != nil {
			log.Println(err)
		}
		c.Env["msg"] = "ShortURL " + fname + " has been deleted"
		http.Redirect(w, r, "/list", http.StatusSeeOther)
	} else {
		fmt.Fprintf(w, "Whatcha trying to do...")
	}
}

/*
func handleAdmin(c web.C, w http.ResponseWriter, r *http.Request) {
    if user, err := aaa.CurrentUser(w, r); err == nil {
        type data struct {
            User httpauth.UserData
            Roles map[string]httpauth.Role
            Users []httpauth.UserData
            Msg []string
        }
        messages := aaa.Messages(w, r)
        users, err := backend.Users()
        if err != nil {
            panic(err)
        }

        d := data{User:user, Roles:roles, Users:users, Msg:messages}
		err = renderTemplate(w, "admin.tmpl", d)
		if err != nil {
			log.Println(err)
		}
	}
}
*/

func APIlgAction(c web.C, w http.ResponseWriter, r *http.Request) {
	url := r.PostFormValue("url")
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
	}
	if r.Form.Get("lg-action") == "ping" {
		//Ping stuff
		out, err := exec.Command("ping", "-c10", url).Output()
		if err != nil {
			log.Println(err)
		}
		outs := string(out)
		title := "Pinging " + url
		p, err := loadPage(title, r, c)
		data := struct {
			Page *Page
		    Title string
		    Message string
		} {
			p,
			title,
			outs,
		}
		err = renderTemplate(w, "lg.tmpl", data)
		if err != nil {
			log.Println(err)
		}
	} else if r.Form.Get("lg-action") == "mtr" {
		//MTR stuff
		out, err := exec.Command("mtr", "--report-wide", "-c10", url).Output()
		if err != nil {
			log.Println(err)
		}
		outs := string(out)
		title := "MTR to " + url
		p, err := loadPage(title, r, c)
		data := struct {
			Page *Page
		    Title string
		    Message string
		} {
			p,
			title,
			outs,
		}
		err = renderTemplate(w, "lg.tmpl", data)
		if err != nil {
			log.Println(err)
		}
	} else if r.Form.Get("lg-action") == "traceroute" {
		//Traceroute stuff
		out, err := exec.Command("traceroute", url).Output()
		if err != nil {
			log.Println(err)
		}
		outs := string(out)
		title := "Traceroute to " + url
		p, err := loadPage(title, r, c)
		data := struct {
			Page *Page
		    Title string
		    Message string
		} {
			p,
			title,
			outs,
		}
		err = renderTemplate(w, "lg.tmpl", data)
		if err != nil {
			log.Println(err)
		}
	} else {
	    //If formvalue isn't MTR, Ping, or traceroute, this should be hit
		http.NotFound(w, r)
		return	    		
	}
}


func APInewSnipForm(c web.C, w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "APInewSnipForm")
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
	}
	title := r.PostFormValue("newsnip")
	http.Redirect(w, r, "/+edit/"+title, http.StatusFound)
	log.Println("New Snip at "+title+" created from search box")
}

//Goji Logger middleware
/*
func LoggerMiddleware(h http.Handler) http.Handler {
	handler := func(w http.ResponseWriter, r *http.Request) {
		rawurl := r.Header.Get("X-Raw-URL")
		ua := r.Header.Get("User-Agent")
		scheme := r.Header.Get("X-Forwarded-Proto")
		ip := r.Header.Get("X-Forwarded-For")
		log.Println("Started "+r.Method+" "+r.URL.Path+"| Host: "+r.Host+" | Raw URL: "+rawurl+" | UserAgent: "+ua+" | HTTPS: "+scheme+" | IP: "+ip) 
		h.ServeHTTP(w, r)
		log.Println("After request")
	}
	return http.HandlerFunc(handler)
}*/



func APInewRemoteImage(c web.C, w http.ResponseWriter, r *http.Request) {
    remoteURL := r.FormValue("remote-image")
    finURL := remoteURL
    if !strings.HasPrefix(remoteURL,"http") {
        log.Println("remoteURL does not contain a URL prefix, so adding http")
        log.Println(remoteURL)
        finURL = "http://"+remoteURL
    }
    fileURL, err := url.Parse(finURL)
    if err != nil {
        panic(err)
    }   
    path := fileURL.Path
    segments := strings.Split(path, "/")
    fileName := segments[len(segments)-1]
    /*
    log.Println("Filename:")
    log.Println(fileName)
    log.Println("Path:")
    log.Println(path)
    */
    dlpath := cfg.ImgDir
    if r.FormValue("remote-image-name") != "" {
    	log.Println("custom remote image name: "+sanitize.Name(r.FormValue("remote-image-name")))
    	fileName = sanitize.Name(r.FormValue("remote-image-name"))
    }
    file, err := os.Create(filepath.Join(dlpath, fileName))
    if err != nil {
        fmt.Println(err)
        panic(err)
    }
    defer file.Close()
    check := http.Client{
            CheckRedirect: func(r *http.Request, via [] *http.Request) error {
            r.URL.Opaque = r.URL.Path
            return nil
            },
    }
    resp, err := check.Get(finURL)
    if err != nil {
        fmt.Println(err)
        panic(err)
    }
    defer resp.Body.Close()
    fmt.Println(resp.Status)

    _, err = io.Copy(file, resp.Body)
    if err != nil {
        panic(err)
    }

    //BoltDB stuff
    imi := &Image{
        Created: time.Now().Unix(),
        Filename: fileName,
        RemoteURL: finURL,
    }
    err = imi.save()
    if err != nil {
        log.Println(err)
    }

    //fmt.Printf("%s with %v bytes downloaded", fileName, size)
    //fmt.Fprintf(w, "%s image with %v bytes downloaded from %s", fileName, size, finURL)
    //fmt.Printf("%s image with %v bytes downloaded from %s", fileName, size, finURL)
    //log.Println("Filename:")
    //log.Println(fileName)
    http.Redirect(w, r, "/i", 302)
}

func APInewImage(c web.C, w http.ResponseWriter, r *http.Request) {
    //vars := mux.Vars(r)
    contentLength := r.ContentLength
    var reader io.Reader
    var f io.WriteCloser
    var err error
    var filename string
    path := cfg.ImgDir
    contentType := r.Header.Get("Content-Type") 
    if contentType == "" {
        log.Println("Content-type blank, so this should be a CLI upload...")
        //Then this should be an upload from the command line...
        reader = r.Body
        if contentLength == -1 {
            var err error
            var f io.Reader
            f = reader
            var b bytes.Buffer
            n, err := io.CopyN(&b, f, _24K+1)
            if err != nil && err != io.EOF {
                log.Printf("%s", err.Error())
                http.Error(w, err.Error(), 500)
                return
            }
            if n > _24K {
                file, err := ioutil.TempFile("./tmp/", "transfer-")
                if err != nil {
                    log.Printf("%s", err.Error())
                    http.Error(w, err.Error(), 500)
                    return
                }
                defer file.Close()
                n, err = io.Copy(file, io.MultiReader(&b, f))
                if err != nil {
                    os.Remove(file.Name())
                    log.Printf("%s", err.Error())
                    http.Error(w, err.Error(), 500)
                    return
                }
                reader, err = os.Open(file.Name())
            } else {
                reader = bytes.NewReader(b.Bytes())
            }
            contentLength = n
        }
        filename = sanitize.Path(filepath.Base(c.URLParams["filename"]))
        if filename == "." {
            log.Println("Filename is blank " + filename)
            dictionary := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
            var bytes = make([]byte, 4)
            rand.Read(bytes)
            for k, v := range bytes {
                bytes[k] = dictionary[v%byte(len(dictionary))]
            }
            filename = string(bytes)
        }
	    if r.FormValue("local-image-name") != "" {
	    	log.Println("custom local image name: "+sanitize.Name(r.FormValue("local-image-name")))
	    	filename = sanitize.Name(r.FormValue("local-image-name"))
	    }
        log.Printf("Uploading image %s %d %s", filename, contentLength, contentType)
        
        if f, err = os.OpenFile(filepath.Join(path, filename), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600); err != nil {
            fmt.Printf("%s", err.Error())
            http.Error(w, errors.New("Could not save image").Error(), 500)
            return
        }
        defer f.Close()
        if _, err = io.Copy(f, reader); err != nil {
            return
        }       
        contentType = mime.TypeByExtension(filepath.Ext(c.URLParams["filename"]))
    } else {
        log.Println("Content-type is "+contentType)
        err := r.ParseMultipartForm(_24K)
        if err != nil {
            log.Println("ParseMultiform reader error")
            log.Println(err)
            return        	
        }
        file, handler, err := r.FormFile("file")
        filename = handler.Filename
        if r.FormValue("local-image-name") != "" {
        	log.Println("CUSTOM FILENAME: ")
        	log.Println(r.FormValue("local-image-name"))
        	filename = r.FormValue("local-image-name")
        }
        if err != nil {
            fmt.Println(err)
            return
        }
        defer file.Close()
        f, err := os.OpenFile(filepath.Join(path, filename), os.O_WRONLY|os.O_CREATE, 0666)
        if err != nil {
            fmt.Println(err)
            return
        }
        defer f.Close()
        io.Copy(f, file)

        /*
        mr, err := r.MultipartReader()
        if err != nil {
            log.Println("Multipart reader error")
            log.Println(err)
            return
        }
        //filename := mr.currentPart.FileHeader.Filename
        //log.Println(r.PostFormValue("local-image-name"))
        for {

            part, err := mr.NextPart()
            if err == io.EOF {
                break
            }
            //if part.FileName() is empty, skip this iteration.
            if part.FileName() != "" {
                filename = part.FileName()
            }
            var read int64
            var p float32
            dst, err := os.OpenFile(filepath.Join(path, filename), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
            if err != nil {
                return
            }
            for {
                buffer := make([]byte, 100000)
                cBytes, err := part.Read(buffer)
                if err == io.EOF {
                    break
                }
                read = read + int64(cBytes)
                //fmt.Printf("read: %v \n",read )
                p = float32(read) / float32(contentLength) *100
                fmt.Fprintf(w, "progress: %v \n",p )
                dst.Write(buffer[0:cBytes])
            }
        }*/

    }

    // w.Statuscode = 200

    //BoltDB stuff
    imi := &Image{
        Created: time.Now().Unix(),
        Filename: filename,
    }
    err = imi.save()
    if err != nil {
        log.Println(err)
    }

	c.Env["msg"] = filename+" successfully uploaded! | <a style='color:#fff' href=/i/"+filename+"><i class='fa fa-link'></i>Link</a>"
	title := filename+" successfully uploaded!"
	p, _ := loadMainPage(title, r, c)
	w.Header().Set("Location", "http://go.jba.io/iup")
	w.WriteHeader(303)
	err = renderTemplate(w, "upimg.tmpl", p)
	if err != nil {
		log.Println(err)
	}
}

func (i *Image) save() error {
    err := Db.Update(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte("Images"))
        encoded, err := json.Marshal(i)
        if err != nil {
        	log.Println(err)
            return err
        }
        return b.Put([]byte(i.Filename), encoded)
    })
    if err != nil {
    	log.Println(err)
    	return err
    }    
    //Detect what kind of image, so we can embiggen GIFs from the get-go

    contentType := mime.TypeByExtension(filepath.Ext(i.Filename))
    if contentType == "image/gif" {
    	log.Println("GIF detected; Running embiggen function...")
    	go embiggenHandler(i.Filename)
    }
    //log.Println(contentType)
    log.Println("+IMAGE SAVED")
    return nil
}


//Goji Custom Logging Middleware
func LoggerMiddleware(c *web.C, h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		reqID := middleware.GetReqID(*c)
		//printStart(reqID, r)
		if reqID != "" {
			fmt.Fprintf(&buf, "[%s] ", reqID)
		}
		buf.WriteString("Started ")
		fmt.Fprintf(&buf, "%s ", r.Method)
		fmt.Fprintf(&buf, "%q ", r.URL.String())
		fmt.Fprintf(&buf, "|Host: %s |RawURL: %s |UserAgent: %s |Scheme: %s |IP: %s ", r.Host, r.Header.Get("X-Raw-URL"), r.Header.Get("User-Agent"), getScheme(r), r.Header.Get("X-Forwarded-For"))
		buf.WriteString("from ")
		buf.WriteString(r.RemoteAddr)

		//Log to file
		f, err := os.OpenFile("./req.log", os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
		if err != nil {
		    log.Fatalf("error opening file: %v", err)
		}
		defer f.Close()
		log.SetOutput(io.MultiWriter(os.Stdout, f))
		log.Print(buf.String())
		//Reset buffer to be reused by the end stuff
		buf.Reset()	

		lw := mutil.WrapWriter(w)

		t1 := time.Now()
		h.ServeHTTP(lw, r)

		if lw.Status() == 0 {
			lw.WriteHeader(http.StatusOK)
		}
		t2 := time.Now()

		//printEnd(reqID, lw, t2.Sub(t1))
		dt := t2.Sub(t1)
		buf.WriteString("Returning ")
		status := lw.Status()
		fmt.Fprintf(&buf, "%v", status)
		buf.WriteString(" in ")
		if dt < 500*time.Millisecond {
			fmt.Fprintf(&buf, "%s", dt)
		} else if dt < 5*time.Second {
			fmt.Fprintf(&buf, "%s", dt)
		} else {
			fmt.Fprintf(&buf, "%s", dt)
		}
		//log.SetOutput(io.MultiWriter(os.Stdout, f))
		log.Print(buf.String())
	}
	return http.HandlerFunc(fn)
}

func viewMarkdownHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	//vars := mux.Vars(r)
    //name := vars["page"]
    name := c.URLParams["page"]
	p, err := loadPage(name, r, c)
	if err != nil {
		http.NotFound(w, r)
		return
	}

    body, err := ioutil.ReadFile("./md/"+name+".md")
	if err != nil {
		http.NotFound(w, r)
		log.Println(err)
		return
	}    
	//unsafe := blackfriday.MarkdownCommon(body)
	md := markdownRender(body) 
	mdhtml := template.HTML(md)
	//html := bluemonday.UGCPolicy().SanitizeBytes(unsafe)	

	data := struct {
		Page *Page
	    Title string
	    MD template.HTML
	} {
		p,
		name,
		mdhtml,
	}
	err = renderTemplate(w, "md.tmpl", data)	
    if err != nil {
    	log.Println(err)
    }
	log.Println(name + " Page rendered!")
}

func Readme(c web.C, w http.ResponseWriter, r *http.Request) {
    name := "README"
	p, err := loadPage(name, r, c)
	if err != nil {
		http.NotFound(w, r)
		return
	}
    body, err := ioutil.ReadFile("./"+name+".md")
	if err != nil {
		log.Println(err)
		return
	}    
	//unsafe := blackfriday.MarkdownCommon(body)
	md := markdownRender(body) 
	mdhtml := template.HTML(md)
	//html := bluemonday.UGCPolicy().SanitizeBytes(unsafe)	
	data := struct {
		Page *Page
	    Title string
	    MD template.HTML
	} {
		p,
		name,
		mdhtml,
	}
	err = renderTemplate(w, "md.tmpl", data)	
    if err != nil {
    	log.Println(err)
    }
	log.Println(name + " Page rendered!")
}

func Changelog(c web.C, w http.ResponseWriter, r *http.Request) {
    name := "CHANGELOG"
	p, err := loadPage(name, r, c)
	if err != nil {
		http.NotFound(w, r)
		return
	}
    body, err := ioutil.ReadFile("./"+name+".md")
	if err != nil {
		log.Println(err)
		return
	}    
	//unsafe := blackfriday.MarkdownCommon(body)
	md := markdownRender(body) 
	mdhtml := template.HTML(md)
	//html := bluemonday.UGCPolicy().SanitizeBytes(unsafe)	
	data := struct {
		Page *Page
	    Title string
	    MD template.HTML
	} {
		p,
		name,
		mdhtml,
	}
	err = renderTemplate(w, "md.tmpl", data)	
    if err != nil {
    	log.Println(err)
    }
	log.Println(name + " Page rendered!")
}

//Generate a random key of specific length
func RandKey(leng int8) string {
	dictionary := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	rb := make([]byte, leng)
	rand.Read(rb)
	for k, v := range rb {
		rb[k] = dictionary[v%byte(len(dictionary))]
	}
	sess_id := string(rb)
	return sess_id
}

//Generate stats, printed in format directly compatible with http://prometheus.io
//I could use the Prometheus client library, but seeing as the runtime stats and boltdb stats provide all I need, I see no point
/*
func runtimeStatsHandler(c web.C, w http.ResponseWriter, r *http.Request) {

	memStats := &runtime.MemStats{}

	nsInMs := float64(time.Millisecond)

	runtime.ReadMemStats(memStats)

	//now := time.Now()

	//How much stuff is being held, taken from BoltDB buckets
	ds := Db.Stats()
	dst := ds.TxStats
	
	fmt.Fprintf(w, "tkot_bolt_tx_num %v\n", ds.TxN)
	fmt.Fprintf(w, "tkot_bolt_tx_page_count %v\n", dst.PageCount)
	fmt.Fprintf(w, "tkot_bolt_tx_cursor_count %v\n", dst.CursorCount)
	fmt.Fprintf(w, "tkot_bolt_tx_write_count %v\n", dst.Write)
	fmt.Fprintf(w, "tkot_bolt_tx_write_time %v\n", dst.WriteTime)
	


	err := Db.View(func(tx *bolt.Tx) error {
		p := tx.Bucket([]byte("Pastes"))
		ps := p.Stats()		
    	f := tx.Bucket([]byte("Files"))
    	fs := f.Stats()
    	s := tx.Bucket([]byte("Snips"))
    	ss := s.Stats()
    	sh := tx.Bucket([]byte("Shorturls"))
    	shs := sh.Stats()
    	i := tx.Bucket([]byte("Images"))
    	is := i.Stats()

		paste_count.Set(float64(ps.KeyN))
		file_count.Set(float64(fs.KeyN))
		snips_count.Set(float64(ss.KeyN))
		shorturl_count.Set(float64(shs.KeyN))
		images_count.Set(float64(is.KeyN))

    	
		fmt.Fprintf(w, "tkot_pastes_total %v\n", ps.KeyN)
		fmt.Fprintf(w, "tkot_files_total %v\n", fs.KeyN)
		fmt.Fprintf(w, "tkot_snips_total %v\n", ss.KeyN)
		fmt.Fprintf(w, "tkot_shorturls_total %v\n", shs.KeyN)
		fmt.Fprintf(w, "tkot_images_total %v\n", is.KeyN)
		
		return nil
	})
	if err != nil {
		log.Println(err)
	}

	
	//Runtime stats
	fmt.Fprintf(w, "tkot_goroutines %v\n", float64(runtime.NumGoroutine()))
	fmt.Fprintf(w, "tkot_memory_allocated %v\n", float64(memStats.Alloc))
	fmt.Fprintf(w, "tkot_memory_mallocs %v \n", float64(memStats.Mallocs))
	fmt.Fprintf(w, "tkot_memory_frees %v \n", float64(memStats.Frees))
	fmt.Fprintf(w, "tkot_memory_gc_total_pause %v \n", float64(memStats.PauseTotalNs)/nsInMs)
	fmt.Fprintf(w, "tkot_memory_heap %v \n", float64(memStats.HeapAlloc))
	fmt.Fprintf(w, "tkot_memory_stack %v \n", float64(memStats.StackInuse))
	fmt.Fprintf(w, "tkot_memory_gc_num %v \n", int(memStats.NumGC))
	

	tx_num.Set(float64(ds.TxN))
	tx_page_count.Set(float64(dst.PageCount))
	tx_cursor_count.Set(float64(dst.CursorCount))
	tx_write_count.Set(float64(dst.Write))
	tx_write_time.Set(float64(dst.WriteTime))
	goroutine_count.Set(float64(runtime.NumGoroutine()))
	memory_allocated.Set(float64(memStats.Alloc))
	memory_mallocs.Set(float64(memStats.Mallocs))
	memory_frees.Set(float64(memStats.Frees))
	memory_gc_total_pause.Set(float64(memStats.PauseTotalNs)/nsInMs)
	memory_heap.Set(float64(memStats.HeapAlloc))
	memory_stack.Set(float64(memStats.StackInuse))
	memory_gc_num.Set(float64(memStats.NumGC))


  
}
*/

func main() {
	/* for reference
	p1 := &Page{Title: "TestPage", Body: []byte("This is a sample page.")}
	p1.save()
	p2, _ := loadPage("TestPage")
	fmt.Println(string(p2.Body))
	*/
	//t := time.Now().Unix()
	//tm := time.Unix(t, 0)
	//log.Println(t)
	//log.Println(tm)
	//log.Println(tm.Format(timestamp))


	//Load conf.json
	conf, _ := os.Open("conf.json")
	decoder := json.NewDecoder(conf)
	err := decoder.Decode(&cfg)
	if err != nil {
		fmt.Println("error decoding config:", err)
	}
	//log.Println(cfg.Username)

	//Check for essential directory existence
   _, err = os.Stat(cfg.ImgDir)
   if err != nil {
      os.Mkdir(cfg.ImgDir, 0755)
   }
   _, err = os.Stat(cfg.FileDir)
   if err != nil {
      os.Mkdir(cfg.FileDir, 0755)
   }
   _, err = os.Stat(cfg.GifDir)
   if err != nil {
      os.Mkdir(cfg.GifDir, 0755)
   }
   _, err = os.Stat(cfg.ThumbDir)
   if err != nil {
      os.Mkdir(cfg.ThumbDir, 0755)
   }

	//var db, _ = bolt.Open("./bolt.db", 0600, nil)
	defer Db.Close()

	Db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("Pastes"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte("Files"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte("Snips"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte("Shorturls"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte("Images"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}		
		return nil
	})
	/*
	Db.View(func(tx *bolt.Tx) error {
    	b := tx.Bucket([]byte("Pastes"))
    	log.Println("-------BOLTDB Pastes: ")
    	b.ForEach(func(k, v []byte) error {
        	fmt.Printf("key=%s, value=%s\n", k, v)
        	return nil
    	})
    	c := tx.Bucket([]byte("Files"))
    	log.Println("-------BOLTDB Files: ")
    	c.ForEach(func(k, v []byte) error {
        	fmt.Printf("key=%s, value=%s\n", k, v)
        	return nil
    	})
    	d := tx.Bucket([]byte("Snips"))
    	log.Println("-------BOLTDB Snips: ")
    	d.ForEach(func(k, v []byte) error {
        	fmt.Printf("key=%s, value=%s\n", k, v)
        	return nil
    	})
    	e := tx.Bucket([]byte("Shorturls"))
    	log.Println("-------BOLTDB Shorturls: ")
    	e.ForEach(func(k, v []byte) error {
        	fmt.Printf("key=%s, value=%s\n", k, v)
        	return nil
    	})
    	f := tx.Bucket([]byte("Images"))
    	log.Println("-------BOLTDB Images: ")
    	f.ForEach(func(k, v []byte) error {
        	fmt.Printf("key=%s, value=%s\n", k, v)
        	return nil
    	})    	
    	return nil
	})*/

	port := os.Getenv("PORT")
	if port == "" {
		port = cfg.Port
	}

	/*
	dictionary := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	rb := make([]byte, 32)
	rand.Read(rb)
	for k, v := range rb {
		rb[k] = dictionary[v%byte(len(dictionary))]
	}
	sess_id := string(rb)
	*/
	new_sess := RandKey(32)
	log.Println("Session ID: " + new_sess)


	memStats := &runtime.MemStats{}

	nsInMs := float64(time.Millisecond)

	runtime.ReadMemStats(memStats)

	//now := time.Now()

	//How much stuff is being held, taken from BoltDB buckets
	ds := Db.Stats()
	dst := ds.TxStats
	/*
	fmt.Fprintf(w, "tkot_bolt_tx_num %v\n", ds.TxN)
	fmt.Fprintf(w, "tkot_bolt_tx_page_count %v\n", dst.PageCount)
	fmt.Fprintf(w, "tkot_bolt_tx_cursor_count %v\n", dst.CursorCount)
	fmt.Fprintf(w, "tkot_bolt_tx_write_count %v\n", dst.Write)
	fmt.Fprintf(w, "tkot_bolt_tx_write_time %v\n", dst.WriteTime)
	*/


	err = Db.View(func(tx *bolt.Tx) error {
		p := tx.Bucket([]byte("Pastes"))
		ps := p.Stats()		
    	f := tx.Bucket([]byte("Files"))
    	fs := f.Stats()
    	s := tx.Bucket([]byte("Snips"))
    	ss := s.Stats()
    	sh := tx.Bucket([]byte("Shorturls"))
    	shs := sh.Stats()
    	i := tx.Bucket([]byte("Images"))
    	is := i.Stats()

		paste_count.Set(float64(ps.KeyN))
		file_count.Set(float64(fs.KeyN))
		snips_count.Set(float64(ss.KeyN))
		shorturl_count.Set(float64(shs.KeyN))
		images_count.Set(float64(is.KeyN))

    	/*
		fmt.Fprintf(w, "tkot_pastes_total %v\n", ps.KeyN)
		fmt.Fprintf(w, "tkot_files_total %v\n", fs.KeyN)
		fmt.Fprintf(w, "tkot_snips_total %v\n", ss.KeyN)
		fmt.Fprintf(w, "tkot_shorturls_total %v\n", shs.KeyN)
		fmt.Fprintf(w, "tkot_images_total %v\n", is.KeyN)
		*/
		return nil
	})
	if err != nil {
		log.Println(err)
	}

	/*
	//Runtime stats
	fmt.Fprintf(w, "tkot_goroutines %v\n", float64(runtime.NumGoroutine()))
	fmt.Fprintf(w, "tkot_memory_allocated %v\n", float64(memStats.Alloc))
	fmt.Fprintf(w, "tkot_memory_mallocs %v \n", float64(memStats.Mallocs))
	fmt.Fprintf(w, "tkot_memory_frees %v \n", float64(memStats.Frees))
	fmt.Fprintf(w, "tkot_memory_gc_total_pause %v \n", float64(memStats.PauseTotalNs)/nsInMs)
	fmt.Fprintf(w, "tkot_memory_heap %v \n", float64(memStats.HeapAlloc))
	fmt.Fprintf(w, "tkot_memory_stack %v \n", float64(memStats.StackInuse))
	fmt.Fprintf(w, "tkot_memory_gc_num %v \n", int(memStats.NumGC))
	*/

	tx_num.Set(float64(ds.TxN))
	tx_page_count.Set(float64(dst.PageCount))
	tx_cursor_count.Set(float64(dst.CursorCount))
	tx_write_count.Set(float64(dst.Write))
	tx_write_time.Set(float64(dst.WriteTime))
	goroutine_count.Set(float64(runtime.NumGoroutine()))
	memory_allocated.Set(float64(memStats.Alloc))
	memory_mallocs.Set(float64(memStats.Mallocs))
	memory_frees.Set(float64(memStats.Frees))
	memory_gc_total_pause.Set(float64(memStats.PauseTotalNs)/nsInMs)
	memory_heap.Set(float64(memStats.HeapAlloc))
	memory_stack.Set(float64(memStats.StackInuse))
	memory_gc_num.Set(float64(memStats.NumGC))

	prometheus.MustRegister(tx_num)
	prometheus.MustRegister(tx_page_count)
	prometheus.MustRegister(tx_cursor_count)
	prometheus.MustRegister(tx_write_count)
	prometheus.MustRegister(paste_count)
	prometheus.MustRegister(snips_count)
	prometheus.MustRegister(file_count)
	prometheus.MustRegister(shorturl_count)
	prometheus.MustRegister(images_count)
	prometheus.MustRegister(goroutine_count)
	prometheus.MustRegister(memory_allocated)
	prometheus.MustRegister(memory_mallocs)
	prometheus.MustRegister(memory_frees)
	prometheus.MustRegister(memory_gc_total_pause)
	prometheus.MustRegister(memory_heap)
	prometheus.MustRegister(memory_stack)
	prometheus.MustRegister(memory_gc_num)


	flag.Parse()
	flag.Set("bind", ":3000")
	
	g := web.New()
	g.Use(middleware.EnvInit)
	//g.Use(AuthMiddleware)
	//g.Abandon(AuthMiddleware)
	g.Use(middleware.RequestID)
    g.Use(LoggerMiddleware)
    g.Use(middleware.Recoverer)
    g.Use(middleware.AutomaticOptions)		
	//Static handler
	g.Use(gojistatic.Static("public", gojistatic.StaticOptions{SkipLogging: true}))
	g.Get("/", indexHandler)

	g.Use(AuthMiddleware)
	g.Get("/priv", Readme)
	g.Abandon(AuthMiddleware)

	g.Get("/readme", Readme)
	g.Get("/changelog", Changelog)
	//Runtime stats
	//g.Get("/stats", runtimeStatsHandler)

	//Login/logout
	g.Post("/login", loginHandler)
	g.Get("/login", loginPageHandler)
	g.Get("/logout", logoutHandler)
	g.Post("/logout", logoutHandler)

	//Protected Functions:

	//g.Use(AuthMiddleware)
	//Edit Snippet
	g.Get("/n/+edit/:page", editSnipHandler)
	//g.Abandon(AuthMiddleware)

	//List of everything
	g.Get("/list", listHandler)
	//Raw snippet page
	g.Get("/n/+raw/:page", rawSnipHandler)
	//New short URL page
	g.Get("/s", shortenPageHandler)
	g.Get("/short", shortenPageHandler)	
	//Looking Glass page
	g.Get("/lg", lgHandler)
	//New Paste Page
	g.Get("/p", pastePageHandler)
	//View existing Paste page
	g.Get("/p/:id", pasteHandler)
	//New Upload Page
	g.Get("/up", uploadPageHandler)
	//New Image Upload Page
	g.Get("/iup", uploadImagePageHandler)
	//Search page
	g.Handle("/search/:term", searchHandler)
	//Download files
	g.Get("/d/:name", downloadHandler)
	//Download BIG images
	g.Get("/big/:name", imageBigHandler)		
	//Download images
	g.Get("/i/:name", downloadImageHandler)	
	//Markdown rendering
	g.Get("/md/:page", viewMarkdownHandler)
	//Thumbs
	g.Get("/thumbs/:name", imageThumbHandler)
	//No hit images
	g.Get("/imagedirect/:name", imageDirectHandler)		
	//Image Gallery
	g.Get("/i", galleryHandler)
	//Image Gallery
	g.Get("/il", galleryListHandler)

	//Test Goji Context
	g.Get("/c-test",  func(c web.C, w http.ResponseWriter, r *http.Request) {
		username := GetUsername(r, c)
		c.Env["user"] = username
		log.Println("c-Env:")
		log.Println(c.Env)
		log.Println(c.Env["user"])
		if user, ok := c.Env["user"].(string); ok {
			w.Write([]byte("Hello " + user))
		} else {
			w.Write([]byte("Hello Stranger!"))
			//log.Println(username)
			//log.Println(c.Env)
			log.Println(c.Env["user"].(string))
		}
	})

	//View Snippet 
	g.Get("/n/:page", snipHandler) 

	//File upload
	g.Post("/up/:id", APInewFile)
	g.Put("/up/:id", APInewFile)
	g.Post("/up", APInewFile)	
	g.Put("/up", APInewFile)
	//Pastebin upload
	g.Post("/p/:id", APInewPaste)
	g.Put("/p/:id", APInewPaste)
	g.Post("/p", APInewPaste)	
	g.Post("/p/", APInewPaste)
	//Looking Glass
	g.Post("/lg", APIlgAction)
	//Snippet/Note functions
	g.Put("/n/+new", APInewSnipForm)
	g.Post("/n/+new", APInewSnipForm)
	g.Put("/n/+new/:page", APIsaveSnip)
	g.Post("/n/+new/:page", APIsaveSnip)	
	g.Post("/n/+append/:page", APIappendSnip)	
	//API Stuff	
	api := web.New()
	g.Handle("/api/*", api)
	api.Use(middleware.SubRouter)
	api.Use(AuthMiddleware)
	api.Get("/delete/:type/:name", APIdeleteHandler)
	api.Abandon(AuthMiddleware)
	//api.Put("/wiki/new", APInewSnipForm)
	//api.Post("/wiki/new", APInewSnipForm)
	//api.Put("/wiki/new/:page", APIsaveSnip)
	//api.Post("/wiki/new/:page", APIsaveSnip)	
	api.Post("/wiki/append/:page", APIappendSnip)
	api.Post("/paste/new", APInewPasteForm)
	api.Post("/file/new", APInewFile)
	api.Post("/file/remote", APInewRemoteFile)
	api.Post("/shorten/new", APInewShortUrlForm)
	api.Post("/lg", APIlgAction)
	api.Post("/image/new", APInewImage)
	api.Post("/image/remote", APInewRemoteImage)

	//g.Get("/metrics", prometheus.Handler())


	//http.Handle("go.dev/", g)
	if fLocal {
		log.Println("Listening on .dev domains due to -l flag...")
		http.Handle("go.dev/metrics", prometheus.UninstrumentedHandler())	
		http.Handle("go.dev/", prometheus.InstrumentHandler("general",g))
	} else {
		log.Println("Listening on "+cfg.MainTLD+" domain")
		http.Handle(cfg.MainTLD+"/metrics", prometheus.UninstrumentedHandler())	
		http.Handle(cfg.MainTLD+"/", prometheus.InstrumentHandler("general",g))
	}
	//Should be the catchall, sends to shortURL for the time being
	//Unsure how to combine Gorilla Mux's wildcard subdomain matching and Goji yet :(
	//goji.Use(LoggerMiddleware)
	//goji.Get("/", shortUrlHandler)
	//goji.Serve()

	//Dedicated image subdomain routes
	i := web.New()
	i.Use(middleware.EnvInit)
	i.Use(middleware.RequestID)
    i.Use(LoggerMiddleware)
    i.Use(middleware.Recoverer)
    i.Use(middleware.AutomaticOptions)		
	//Static handler
	i.Use(gojistatic.Static("public", gojistatic.StaticOptions{SkipLogging: true}))
	i.Get("/", galleryEsgyHandler)	
	//Thumbs
	i.Get("/thumbs/:name", imageThumbHandler)
	//No hit images
	i.Get("/imagedirect/:name", imageDirectHandler)	
	//Huge images
	i.Get("/big/:name", imageBigHandler)		
	//Download images
	i.Get("/:name", downloadImageHandler)
	http.Handle(cfg.ImageTLD+"/", prometheus.InstrumentHandler("images",i))

	//Dedicated BIG image subdomain for easy linking
	big := web.New()
	big.Use(middleware.EnvInit)
	//g.Use(AuthMiddleware)
	//g.Abandon(AuthMiddleware)
	big.Use(middleware.RequestID)
    big.Use(LoggerMiddleware)
    big.Use(middleware.Recoverer)
    big.Use(middleware.AutomaticOptions)	
	big.Use(gojistatic.Static("public", gojistatic.StaticOptions{SkipLogging: true}))    	
	//Huge images
	big.Get("/:name", imageBigHandler)	
	http.Handle(cfg.GifTLD+"/", prometheus.InstrumentHandler("big_gifs", big))

	//Default Goji mux which picks up all requests leftover and directs them to shortURLHandler
	mygoji := web.New()
	mygoji.Use(middleware.RequestID)
    mygoji.Use(LoggerMiddleware)
    mygoji.Use(middleware.Recoverer)
    mygoji.Use(middleware.AutomaticOptions)	
    mygoji.Get("/", shortUrlHandler)

    mygoji.Compile()
	api.Compile()
	g.Compile()   
	i.Compile()	 
	big.Compile()

    http.Handle("/", prometheus.InstrumentHandler("shorturls",mygoji))
    listener := bind.Default()
    log.Println("Starting Goji on", listener.Addr())
	graceful.HandleSignals()
	bind.Ready()
	graceful.PreHook(func() { log.Printf("Goji received signal, gracefully stopping") })
	graceful.PostHook(func() { log.Printf("Goji stopped") })
	err = graceful.Serve(listener, http.DefaultServeMux)
	if err != nil {
		log.Fatal(err)
	}
	graceful.Wait()

}
