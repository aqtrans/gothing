package main

// TODO
// - Guard file/image upload pages from respective filetypes
// - Add a screenshot sharing route, separate from image gallery
// - Refactor all save() functions to do the actual file saving as well...
// ...only saving if the BoltDB function doesn't error out
//blah

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/disintegration/imaging"
	"github.com/gorilla/mux"
	"github.com/justinas/alice"
	"github.com/oxtoacart/bpool"
	//"github.com/prometheus/client_golang/prometheus"
    "github.com/fukata/golang-stats-api-handler"
	"github.com/russross/blackfriday"
	"html/template"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	//"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
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
	LDAPport uint16
	LDAPurl  string
	LDAPdn   string
	LDAPun   string
}

var (
	bufpool   *bpool.BufferPool
	templates map[string]*template.Template
	_24K      int64 = (1 << 20) * 24
	fLocal    bool
	debug 	  bool 
	Db, _     = bolt.Open("./bolt.db", 0600, nil)
	cfg       = Configuration{}

)

//Flags
//var fLocal = flag.Bool("l", false, "Turn on localhost resolving for Handlers")

//Base struct, Page ; has to be wrapped in a data {} strut for consistency reasons
type Page struct {
	TheName string
	Title   string
	UN      string
}

type ListPage struct {
	*Page
	Pastes    []*Paste
	Files     []*File
	Shorturls []*Shorturl
	Images    []*Image
}

type GalleryPage struct {
	*Page
	Images []*Image
}

//BoltDB structs:
type Paste struct {
	Created int64
	Title   string
	Content string
	Hits    int64
}

type File struct {
	Created   int64
	Filename  string
	Hits      int64
	RemoteURL string
}

type Image struct {
	Created   int64
	Filename  string
	Hits      int64
	RemoteURL string
}

type Shorturl struct {
	Created int64
	Short   string
	Long    string
	Hits    int64
}

//JSON Response
type jsonresponse struct {
	Name    string `json:"name,omitempty"`
	Success bool   `json:"success"`
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

type FileByDate []*File

func (a FileByDate) Len() int           { return len(a) }
func (a FileByDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a FileByDate) Less(i, j int) bool { return a[i].Created < a[j].Created }

type ShortByDate []*Shorturl

func (a ShortByDate) Len() int           { return len(a) }
func (a ShortByDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ShortByDate) Less(i, j int) bool { return a[i].Created < a[j].Created }

func init() {
	//Flag '-l' enables go.dev and *.dev domain resolution
	flag.BoolVar(&fLocal, "l", false, "Turn on localhost resolving for Handlers")
	//Flag '-d' enabled debug logging
	flag.BoolVar(&debug, "d", false, "Enabled debug logging")

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

	funcMap := template.FuncMap{"prettyDate": PrettyDate, "safeHTML": SafeHTML, "imgClass": ImgClass}

	for _, layout := range layouts {
		files := append(includes, layout)
		//DEBUG TEMPLATE LOADING 
		Debugln(files)
		templates[filepath.Base(layout)] = template.Must(template.New("templates").Funcs(funcMap).ParseFiles(files...))
	}
}

func Debugln(v ...interface{}) {
	if debug {
		d := log.New(os.Stdout, "DEBUG: ", log.Ldate)
		d.Println(v)
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
	scheme = r.Header.Get("X-Forwarded-Proto") + "://"
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

func ParseBool(value string) bool {
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return boolValue
}

func loadPage(title string, r *http.Request) (*Page, error) {
	//timer.Step("loadpageFunc")
	user := GetUsername(r)
	return &Page{TheName: "GoThing", Title: title, UN: user}, nil
}

func loadMainPage(title string, r *http.Request) (interface{}, error) {
	//timer.Step("loadpageFunc")
	p, err := loadPage(title, r)
	if err != nil {
		return nil, err
	}
	data := struct {
		Page *Page
	}{
		p,
	}
	return data, nil
}

func loadListPage(r *http.Request) (*ListPage, error) {
	page, perr := loadPage("List", r)
	if perr != nil {
		return nil, perr
	}

	var files []*File
	//Lets try this with boltDB now!
	Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Files"))
		b.ForEach(func(k, v []byte) error {
			Debugln("FILES: key="+string(k)+" value="+string(v))
			var file *File
			err := json.Unmarshal(v, &file)
			if err != nil {
				log.Panicln(err)
			}
			files = append(files, file)
			return nil
		})
		return nil
	})
	sort.Sort(FileByDate(files))

	var pastes []*Paste
	//Lets try this with boltDB now!
	Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Pastes"))
		b.ForEach(func(k, v []byte) error {
			Debugln("PASTE: key="+string(k)+" value="+string(v))
			var paste *Paste
			err := json.Unmarshal(v, &paste)
			if err != nil {
				log.Panicln(err)
			}
			pastes = append(pastes, paste)
			return nil
		})
		return nil
	})
	sort.Sort(PasteByDate(pastes))

	var shorts []*Shorturl
	//Lets try this with boltDB now!
	Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Shorturls"))
		b.ForEach(func(k, v []byte) error {
			Debugln("SHORT: key="+string(k)+" value="+string(v))
			var short *Shorturl
			err := json.Unmarshal(v, &short)
			if err != nil {
				log.Panicln(err)
			}
			shorts = append(shorts, short)
			return nil
		})
		return nil
	})
	sort.Sort(ShortByDate(shorts))

	var images []*Image
	//Lets try this with boltDB now!
	Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Images"))
		b.ForEach(func(k, v []byte) error {
			Debugln("IMAGE: key="+string(k)+" value="+string(v))
			var image *Image
			err := json.Unmarshal(v, &image)
			if err != nil {
				log.Panicln(err)
			}
			images = append(images, image)
			return nil
		})
		return nil
	})
	sort.Sort(ImageByDate(images))

	return &ListPage{Page: page, Pastes: pastes, Files: files, Shorturls: shorts, Images: images}, nil
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

func (f *File) save() error {
	err := Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Files"))
		encoded, err := json.Marshal(f)
		if err != nil {
			log.Panicln(err)
			return err
		}
		return b.Put([]byte(f.Filename), encoded)
	})
	if err != nil {
		log.Println(err)
		return err
	}
	log.Println("++++FILE SAVED")
	return nil
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

func (p *Paste) save() error {
	err := Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Pastes"))
		encoded, err := json.Marshal(p)
		if err != nil {
			log.Panicln(err)
			return err
		}
		return b.Put([]byte(p.Title), encoded)
	})
	if err != nil {
		log.Panicln(err)
		return err
	}
	log.Println("++++PASTE SAVED")
	return nil
}

func makeThumb(fpath, thumbpath string) {
	contentType := mime.TypeByExtension(filepath.Ext(path.Base(fpath)))
	if contentType == "video/webm" {
		log.Println("WEBM FILE DETECTED")
		//ffmpeg -i doit.webm -vframes 1 -filter:v scale="-1:300" doit.thumb.png
		resize := exec.Command("/usr/bin/ffmpeg", "-i", fpath, "-vframes", "1", "-filter:v", "scale='-1:300'", thumbpath)
		err := resize.Run()
		if err != nil {
			log.Panicln(err)
		}
		return
	}

	img, err := imaging.Open(fpath)
	if err != nil {
		log.Panicln(err)
		return
	}
	thumb := imaging.Fit(img, 600, 300, imaging.CatmullRom)
	err = imaging.Save(thumb, thumbpath)
	if err != nil {
		log.Panicln(err)
		return
	}
	return
}

func (i *Image) save() error {
	err := Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Images"))
		encoded, err := json.Marshal(i)
		if err != nil {
			log.Panicln(err)
			return err
		}
		return b.Put([]byte(i.Filename), encoded)
	})
	if err != nil {
		log.Panicln(err)
		return err
	}
	//Detect what kind of image, so we can embiggen GIFs from the get-go

	contentType := mime.TypeByExtension(filepath.Ext(i.Filename))
	if contentType == "image/gif" {
		log.Println("GIF detected; Running embiggen function...")
		go embiggenHandler(i.Filename)
	}
	log.Println("++++IMAGE SAVED")
	return nil
}

type statusWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Status() int {
	return w.status
}

func (w *statusWriter) Size() int {
	return w.size
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	written, err := w.ResponseWriter.Write(b)
	w.size += written
	return written, err
}

//Custom Logging Middleware
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer

		start := time.Now()
		writer := statusWriter{w, 0, 0}

		buf.WriteString("Started ")
		fmt.Fprintf(&buf, "%s ", r.Method)
		fmt.Fprintf(&buf, "%q ", r.URL.String())
		fmt.Fprintf(&buf, "|Host: %s |RawURL: %s |UserAgent: %s |Scheme: %s |IP: %s ", r.Host, r.Header.Get("X-Raw-URL"), r.Header.Get("User-Agent"), getScheme(r), r.Header.Get("X-Forwarded-For"))
		buf.WriteString("from ")
		buf.WriteString(r.RemoteAddr)

		//Log to file
		f, err := os.OpenFile("./req.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		defer f.Close()
		log.SetOutput(io.MultiWriter(os.Stdout, f))
		log.Print(buf.String())
		//Reset buffer to be reused by the end stuff
		buf.Reset()

		next.ServeHTTP(&writer, r)

		end := time.Now()
		latency := end.Sub(start)
		status := writer.Status()

		buf.WriteString("Returning ")
		fmt.Fprintf(&buf, "%v", status)
		buf.WriteString(" in ")
		fmt.Fprintf(&buf, "%s", latency)
		//log.SetOutput(io.MultiWriter(os.Stdout, f))
		log.Print(buf.String())
	})
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

func makeJSON(w http.ResponseWriter, data interface{}) ([]byte, error) {
	jsonData, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return nil, err
	}
	Debugln(string(jsonData))
	return jsonData, nil
}

func WriteJ(w http.ResponseWriter, name string, success bool) error {
	j := jsonresponse{
		Name:    name,
		Success: success,
	}
	json, err := makeJSON(w, j)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	w.Write(json)
	Debugln(string(json))
	return nil
}

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

	port := os.Getenv("PORT")
	if port == "" {
		port = cfg.Port
	}

	new_sess := RandKey(32)
	log.Println("Session ID: " + new_sess)

	flag.Parse()
	flag.Set("bind", ":3000")

	std := alice.New(Logger)
	//stda := alice.New(Auth, Logger)

	r := mux.NewRouter().StrictSlash(true)
	d := r.Host("go.jba.io").Subrouter()

	if fLocal {
		log.Println("Listening on .dev domains due to -l flag...")
		d = r.Host("go.dev").Subrouter()
	} else {
		log.Println("Listening on " + cfg.MainTLD + " domain")
	}

	d.HandleFunc("/", indexHandler).Methods("GET")
	d.HandleFunc("/priv", Auth(Readme)).Methods("GET")
	d.HandleFunc("/readme", Readme).Methods("GET")
	d.HandleFunc("/changelog", Changelog).Methods("GET")
	d.HandleFunc("/login", loginHandler).Methods("POST")
	d.HandleFunc("/login", loginPageHandler).Methods("GET")
	d.HandleFunc("/logout", logoutHandler).Methods("POST")
	d.HandleFunc("/logout", logoutHandler).Methods("GET")
	d.HandleFunc("/list", Auth(listHandler)).Methods("GET")
	d.HandleFunc("/s", Auth(shortenPageHandler)).Methods("GET")
	d.HandleFunc("/short", Auth(shortenPageHandler)).Methods("GET")
	d.HandleFunc("/lg", lgHandler).Methods("GET")
	d.HandleFunc("/p", pastePageHandler).Methods("GET")
	d.HandleFunc("/p/{name}", pasteHandler).Methods("GET")
	d.HandleFunc("/up", uploadPageHandler).Methods("GET")
	d.HandleFunc("/iup", uploadImagePageHandler).Methods("GET")
	d.HandleFunc("/search/{name}", Auth(searchHandler)).Methods("GET")
	d.HandleFunc("/d/{name}", downloadHandler).Methods("GET")
	d.HandleFunc("/big/{name}", imageBigHandler).Methods("GET")
	d.HandleFunc("/i/{name}", downloadImageHandler).Methods("GET")
	d.HandleFunc("/md/{name}", viewMarkdownHandler).Methods("GET")
	d.HandleFunc("/thumbs/{name}", imageThumbHandler).Methods("GET")
	d.HandleFunc("/imagedirect/{name}", imageDirectHandler).Methods("GET")
	d.HandleFunc("/i", galleryHandler).Methods("GET")
	d.HandleFunc("/il", galleryListHandler).Methods("GET")
	d.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		WriteJ(w, "LOL", false)
	}).Methods("GET", "POST")
	d.HandleFunc("/json2", func(w http.ResponseWriter, r *http.Request) {
		WriteJ(w, "", false)
	}).Methods("GET", "POST")

	//CLI API Functions
	d.HandleFunc("/up/{name}", APInewFile).Methods("POST", "PUT")
	d.HandleFunc("/up", APInewFile).Methods("POST", "PUT")
	d.HandleFunc("/p/{name}", APInewPaste).Methods("POST", "PUT")
	d.HandleFunc("/p", APInewPaste).Methods("POST", "PUT")
	d.HandleFunc("/lg", APIlgAction).Methods("POST")

	//API Functions
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/delete/{type}/{name}", Auth(APIdeleteHandler)).Methods("GET")
	api.HandleFunc("/paste/new", APInewPasteForm).Methods("POST")
	api.HandleFunc("/file/new", APInewFile).Methods("POST")
	api.HandleFunc("/file/remote", APInewRemoteFile).Methods("POST")
	api.HandleFunc("/shorten/new", APInewShortUrlForm).Methods("POST")
	api.HandleFunc("/lg", APIlgAction).Methods("POST")
	api.HandleFunc("/image/new", APInewImage).Methods("POST")
	api.HandleFunc("/image/remote", APInewRemoteImage).Methods("POST")
    //Golang-Stats-API
    api.HandleFunc("/stats", stats_api.Handler)

	//Dedicated image subdomain routes
	i := r.Host(cfg.ImageTLD).Subrouter()
	i.HandleFunc("/", galleryEsgyHandler).Methods("GET")
	i.HandleFunc("/thumbs/{name}", imageThumbHandler).Methods("GET")
	i.HandleFunc("/imagedirect/{name}", imageDirectHandler).Methods("GET")
	i.HandleFunc("/big/{name}", imageBigHandler).Methods("GET")
	i.HandleFunc("/{name}", downloadImageHandler).Methods("GET")

	//Big GIFs
	big := r.Host(cfg.GifTLD).Subrouter()
	big.HandleFunc("/{name}", imageBigHandler).Methods("GET")

	//Dynamic subdomains
	wild := r.Host("{name}.es.gy").Subrouter()
	wild.HandleFunc("/", shortUrlHandler).Methods("GET")

	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./public/")))
	http.Handle("/", std.Then(r))
	http.ListenAndServe(":3000", nil)

	//Runtime stats
	//g.Get("/stats", runtimeStatsHandler)

	//Test Goji Context
	/*r.GET("/c-test",  func(w http.ResponseWriter, r *http.Request) {
		username := GetUsername(c)
		c.Get("user") = username
		log.Println("c-Env:")
		log.Println(c.Keys)
		log.Println(c.Get("user"))
		if user, ok := c.Get("user").(string); ok {
			w.Write([]byte("Hello " + user))
		} else {
			w.Write([]byte("Hello Stranger!"))
			//log.Println(username)
			//log.Println(c.Env)
			log.Println(c.Get("user").(string))
		}
	})*/

}
