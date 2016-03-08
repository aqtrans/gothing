package main

// TODO
// - Guard file/image upload pages from respective filetypes
// - Add a screenshot sharing route, separate from image gallery
// - Refactor all save() functions to do the actual file saving as well...
// ...only saving if the BoltDB function doesn't error out
//blah

import (
	"encoding/json"
	"flag"
	"fmt"
	//"expvar"
	//"runtime"
	"github.com/boltdb/bolt"
	"github.com/disintegration/imaging"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/justinas/alice"
	"github.com/oxtoacart/bpool"
	//"github.com/fukata/golang-stats-api-handler"
	"github.com/russross/blackfriday"
	"html/template"
	"jba.io/go/auth"
	"jba.io/go/utils"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

type configuration struct {
	Port     string
	Email    string
	ImgDir   string
	FileDir  string
	ThumbDir string
	GifDir   string
	MainTLD  string
	ShortTLD string
	ImageTLD string
	GifTLD   string
	AuthConf auth.AuthConf
}

var (
	bufpool   *bpool.BufferPool
	templates map[string]*template.Template
	_24K      int64 = (1 << 20) * 24
	fLocal    bool
	debug     bool
	db, _     = bolt.Open("./bolt.db", 0600, nil)
	cfg       = configuration{}
)

//Flags
//var fLocal = flag.Bool("l", false, "Turn on localhost resolving for Handlers")

//Page has to be wrapped in a data {} strut for consistency reasons
type page struct {
	TheName string
	Title   string
	UN      string
	Token   string
}

type listPage struct {
	*page
	Pastes    []*paste
	Files     []*file
	Shorturls []*shorturl
	Images    []*image
}

type galleryPage struct {
	*page
	Images []*image
}

//BoltDB structs:
type paste struct {
	Created int64
	Title   string
	Content string
	Hits    int64
}

type file struct {
	Created   int64
	Filename  string
	Hits      int64
	RemoteURL string
}

type image struct {
	Created   int64
	Filename  string
	Hits      int64
	RemoteURL string
}

type screenshots struct {
	Created   int64
	Filename  string
	Hits      int64
	RemoteURL string
}

type shorturl struct {
	Created int64
	Short   string
	FullURL string
	Long    string
	Hits    int64
}

// Sorting functions
type ImageByDate []*image

func (a ImageByDate) Len() int           { return len(a) }
func (a ImageByDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ImageByDate) Less(i, j int) bool { return a[i].Created < a[j].Created }

type PasteByDate []*paste

func (a PasteByDate) Len() int           { return len(a) }
func (a PasteByDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a PasteByDate) Less(i, j int) bool { return a[i].Created < a[j].Created }

type FileByDate []*file

func (a FileByDate) Len() int           { return len(a) }
func (a FileByDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a FileByDate) Less(i, j int) bool { return a[i].Created < a[j].Created }

type ShortByDate []*shorturl

func (a ShortByDate) Len() int           { return len(a) }
func (a ShortByDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ShortByDate) Less(i, j int) bool { return a[i].Created < a[j].Created }

func init() {

	//Flag '-l' enables go.dev and *.dev domain resolution
	flag.BoolVar(&fLocal, "l", false, "Turn on localhost resolving for Handlers")
	//Flag '-d' enabled debug logging
	flag.BoolVar(&utils.Debug, "d", false, "Enabled debug logging")

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

	funcMap := template.FuncMap{"prettyDate": utils.PrettyDate, "safeHTML": utils.SafeHTML, "imgClass": utils.ImgClass, "imgExt": utils.ImgExt}

	for _, layout := range layouts {
		files := append(includes, layout)
		//DEBUG TEMPLATE LOADING
		utils.Debugln(files)
		templates[filepath.Base(layout)] = template.Must(template.New("templates").Funcs(funcMap).ParseFiles(files...))
	}
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
	defer utils.TimeTrack(time.Now(), "getScheme")
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
	defer utils.TimeTrack(time.Now(), "renderTemplate")
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
	defer utils.TimeTrack(time.Now(), "ParseBool")
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return boolValue
}

func loadPage(title string, w http.ResponseWriter, r *http.Request) (*page, error) {
	defer utils.TimeTrack(time.Now(), "loadPage")
	//timer.Step("loadpageFunc")
	user := auth.GetUsername(r)
	token := auth.GetToken(r)
	return &page{TheName: "GoThing", Title: title, UN: user, Token: token}, nil
}

func loadMainPage(title string, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	defer utils.TimeTrack(time.Now(), "loadMainPage")
	//timer.Step("loadpageFunc")
	p, err := loadPage(title, w, r)
	if err != nil {
		return nil, err
	}
	data := struct {
		Page *page
	}{
		p,
	}
	return data, nil
}

func loadListPage(w http.ResponseWriter, r *http.Request) (*listPage, error) {
	defer utils.TimeTrack(time.Now(), "loadListPage")
	p, perr := loadPage("List", w, r)
	if perr != nil {
		return nil, perr
	}

	var thefiles []*file
	//Lets try this with boltDB now!
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Files"))
		b.ForEach(func(k, v []byte) error {
			utils.Debugln("FILES: key=" + string(k) + " value=" + string(v))
			var thefile *file
			err := json.Unmarshal(v, &thefile)
			if err != nil {
				log.Panicln(err)
			}
			thefiles = append(thefiles, thefile)
			return nil
		})
		return nil
	})
	sort.Sort(FileByDate(thefiles))

	var thepastes []*paste
	//Lets try this with boltDB now!
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Pastes"))
		b.ForEach(func(k, v []byte) error {
			utils.Debugln("PASTE: key=" + string(k) + " value=" + string(v))
			var thepaste *paste
			err := json.Unmarshal(v, &thepaste)
			if err != nil {
				log.Panicln(err)
			}
			thepastes = append(thepastes, thepaste)
			return nil
		})
		return nil
	})
	sort.Sort(PasteByDate(thepastes))

	var theshorts []*shorturl
	//Lets try this with boltDB now!
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Shorturls"))
		b.ForEach(func(k, v []byte) error {
			utils.Debugln("SHORT: key=" + string(k) + " value=" + string(v))
			var theshort *shorturl
			err := json.Unmarshal(v, &theshort)
			if err != nil {
				log.Panicln(err)
			}
			theshorts = append(theshorts, theshort)
			return nil
		})
		return nil
	})
	sort.Sort(ShortByDate(theshorts))

	var theimages []*image
	//Lets try this with boltDB now!
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Images"))
		b.ForEach(func(k, v []byte) error {
			utils.Debugln("IMAGE: key=" + string(k) + " value=" + string(v))
			var theimage *image
			err := json.Unmarshal(v, &theimage)
			if err != nil {
				log.Panicln(err)
			}
			theimages = append(theimages, theimage)
			return nil
		})
		return nil
	})
	sort.Sort(ImageByDate(theimages))

	return &listPage{page: p, Pastes: thepastes, Files: thefiles, Shorturls: theshorts, Images: theimages}, nil
}

func ParseMultipartFormProg(r *http.Request, maxMemory int64) error {
	defer utils.TimeTrack(time.Now(), "ParseMultipartFormProg")
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

func (f *file) save() error {
	defer utils.TimeTrack(time.Now(), "File.save()")
	err := db.Update(func(tx *bolt.Tx) error {
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

func (s *shorturl) save() error {
	defer utils.TimeTrack(time.Now(), "Shorturl.save()")
	err := db.Update(func(tx *bolt.Tx) error {
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

func (p *paste) save() error {
	defer utils.TimeTrack(time.Now(), "Paste.save()")
	err := db.Update(func(tx *bolt.Tx) error {
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
	defer utils.TimeTrack(time.Now(), "makeThumb")
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

func (i *image) save() error {
	defer utils.TimeTrack(time.Now(), "Image.save()")
	err := db.Update(func(tx *bolt.Tx) error {
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
	// No longer needed as of 03/06/2016
	/*
		contentType := mime.TypeByExtension(filepath.Ext(i.Filename))
		if contentType == "image/gif" {
			log.Println("GIF detected; Running embiggen function...")
			go embiggenHandler(i.Filename)
		}
	*/
	log.Println("++++IMAGE SAVED")
	return nil
}

func defaultHandler(next http.Handler) http.Handler {
	defer utils.TimeTrack(time.Now(), "defaultHandler")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host == cfg.ImageTLD || r.Host == cfg.MainTLD || r.Host == "www."+cfg.MainTLD || r.Host == cfg.ShortTLD || r.Host == cfg.GifTLD || r.Host == "go.dev" || r.Host == "go.jba.io" {
			next.ServeHTTP(w, r)
		} else {
			log.Println("Not serving anything, because this request belongs to: " + r.Host)
			http.Error(w, http.StatusText(400), 400)
			return
		}
	})
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
	defer db.Close()

	db.Update(func(tx *bolt.Tx) error {
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
		_, err = tx.CreateBucketIfNotExists([]byte("SubShorturl"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = cfg.Port
	}

	//new_sess := utils.RandKey(32)
	//log.Println("Session ID: " + new_sess)

	flag.Parse()
	flag.Set("bind", ":3000")

	std := alice.New(handlers.RecoveryHandler(), auth.UserEnvMiddle, auth.XsrfMiddle, utils.Logger)
	//stda := alice.New(Auth, Logger)

	r := mux.NewRouter().StrictSlash(true)
	d := r.Host(cfg.MainTLD).Subrouter()

	if fLocal {
		log.Println("Listening on .dev domains due to -l flag...")
		d = r.Host("go.dev").Subrouter()
	} else {
		log.Println("Listening on " + cfg.MainTLD + " domain")
	}

	d.HandleFunc("/", indexHandler).Methods("GET")
	d.HandleFunc("/help", helpHandler).Methods("GET")
	d.HandleFunc("/priv", auth.AuthMiddle(Readme)).Methods("GET")
	d.HandleFunc("/readme", Readme).Methods("GET")
	d.HandleFunc("/changelog", Changelog).Methods("GET")
	d.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) { auth.LoginPostHandler(cfg.AuthConf, w, r) }).Methods("POST")
	d.HandleFunc("/login", loginPageHandler).Methods("GET")
	d.HandleFunc("/logout", auth.LogoutHandler).Methods("POST")
	d.HandleFunc("/logout", auth.LogoutHandler).Methods("GET")

	d.HandleFunc("/list", auth.AuthMiddle(listHandler)).Methods("GET")
	d.HandleFunc("/s", auth.AuthMiddle(shortenPageHandler)).Methods("GET")
	d.HandleFunc("/short", auth.AuthMiddle(shortenPageHandler)).Methods("GET")
	d.HandleFunc("/lg", lgHandler).Methods("GET")
	d.HandleFunc("/p", pastePageHandler).Methods("GET")
	d.HandleFunc("/p/{name}", pasteHandler).Methods("GET")
	d.HandleFunc("/up", uploadPageHandler).Methods("GET")
	d.HandleFunc("/iup", uploadImagePageHandler).Methods("GET")
	d.HandleFunc("/search/{name}", auth.AuthMiddle(searchHandler)).Methods("GET")
	d.HandleFunc("/d/{name}", downloadHandler).Methods("GET")
	d.HandleFunc("/big/{name}", imageBigHandler).Methods("GET")
	d.HandleFunc("/i/{name}", downloadImageHandler).Methods("GET")
	d.HandleFunc("/md/{name}", viewMarkdownHandler).Methods("GET")
	d.HandleFunc("/thumbs/{name}", imageThumbHandler).Methods("GET")
	d.HandleFunc("/imagedirect/{name}", imageDirectHandler).Methods("GET")
	d.HandleFunc("/i", galleryHandler).Methods("GET")
	d.HandleFunc("/il", galleryListHandler).Methods("GET")
	//d.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {utils.WriteJ(w, "LOL", false)}).Methods("GET", "POST")
	//d.HandleFunc("/json2", func(w http.ResponseWriter, r *http.Request) {utils.WriteJ(w, "", false)}).Methods("GET", "POST")

	//CLI API Functions
	d.HandleFunc("/up/{name}", APInewFile).Methods("POST", "PUT")
	d.HandleFunc("/up", APInewFile).Methods("POST", "PUT")
	d.HandleFunc("/p/{name}", APInewPaste).Methods("POST", "PUT")
	d.HandleFunc("/p", APInewPaste).Methods("POST", "PUT")
	d.HandleFunc("/lg", APIlgAction).Methods("POST")

	//API Functions
	api := d.PathPrefix("/api").Subrouter()
	api.HandleFunc("/delete/{type}/{name}", auth.AuthMiddle(APIdeleteHandler)).Methods("GET")
	api.HandleFunc("/paste/new", APInewPasteForm).Methods("POST")
	api.HandleFunc("/file/new", APInewFile).Methods("POST")
	api.HandleFunc("/file/remote", APInewRemoteFile).Methods("POST")
	api.HandleFunc("/shorten/new", APInewShortUrlForm).Methods("POST")
	api.HandleFunc("/lg", APIlgAction).Methods("POST")
	api.HandleFunc("/image/new", APInewImage).Methods("POST")
	api.HandleFunc("/image/remote", APInewRemoteImage).Methods("POST")
	//Golang-Stats-API
	//api.HandleFunc("/stats", stats_api.Handler)
	api.HandleFunc("/vars", utils.HandleExpvars)

	//Dedicated image subdomain routes
	i := r.Host(cfg.ImageTLD).Subrouter()
	i.HandleFunc("/", galleryEsgyHandler).Methods("GET")
	i.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "") })
	i.HandleFunc("/favicon.png", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "") })
	i.HandleFunc("/thumbs/{name}", imageThumbHandler).Methods("GET")
	i.HandleFunc("/imagedirect/{name}", imageDirectHandler).Methods("GET")
	i.HandleFunc("/big/{name}", imageBigHandler).Methods("GET")
	i.HandleFunc("/{name}", downloadImageHandler).Methods("GET")

	//Big GIFs
	big := r.Host(cfg.GifTLD).Subrouter()
	big.HandleFunc("/{name}", imageBigHandler).Methods("GET")

	//Dynamic subdomains | try to avoid taking www.es.gy
	//wild := r.Host("{name:([^www][A-Za-z0-9]+)}.es.gy").Subrouter()
	wild := r.Host("{name}.es.gy").Subrouter()
	wild.HandleFunc("/", shortUrlHandler).Methods("GET")
	//Main Short URL page
	// Collapsing this into main TLD
	//short := r.Host(cfg.ShortTLD).Subrouter()
	//short.HandleFunc("/{name}", shortUrlHandler).Methods("GET")

	//static := http.Handler(http.FileServer(http.Dir("./public/")))

	//r.PathPrefix("/assets/").Handler(defaultHandler(static))
    r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets/"))))
	d.HandleFunc("/{name}", shortUrlHandler).Methods("GET")
	http.Handle("/", std.Then(r))
    http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "./assets/favicon.ico") })
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
