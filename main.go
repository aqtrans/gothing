package main

// Recent changes:
// - Moved from gorilla/mux to httptreemux+go1.7 context

// TODO
// - Guard file/image upload pages from respective filetypes
// - Add a screenshot sharing route, separate from image gallery
// - Refactor all save() functions to do the actual file saving as well...
// ...only saving if the BoltDB function doesn't error out

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/spf13/pflag"

	"html/template"
	"regexp"

	"github.com/boltdb/bolt"
	"github.com/dimfeld/httptreemux"
	"github.com/disintegration/imaging"
	"github.com/gorilla/handlers"
	"github.com/justinas/alice"
	"github.com/oxtoacart/bpool"

	"github.com/gorilla/csrf"
	"github.com/spf13/viper"

	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/russross/blackfriday"

	"jba.io/go/auth"
	"jba.io/go/httputils"
)

type configuration struct {
	Port     string
	Email    string
	ImgDir   string
	FileDir  string
	ThumbDir string
	MainTLD  string
	ShortTLD string
	ImageTLD string
	GifTLD   string
}

type thingEnv struct {
	Bolt      *thingDB
	authState *auth.State
	templates map[string]*template.Template
}

type thingDB struct {
	db   *bolt.DB
	path string
}

var (
	bufpool *bpool.BufferPool
	_24K    int64 = (1 << 20) * 24
	dataDir string
	//db, _     = bolt.Open("./data/bolt.db", 0600, nil)
	//cfg       = configuration{}
)

func (env *thingEnv) getDB() *bolt.DB {
	//log.Println(state.BoltDB.path)
	db, err := bolt.Open(env.Bolt.path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Fatalln(err)
	}
	env.Bolt.db = db
	return env.Bolt.db
}

func (env *thingEnv) closeDB() {
	env.Bolt.db.Close()
}

// ReCAPTCHA from https://github.com/dasJ/go-recaptcha/blob/440394abc3ecd036b93a54837015d5fe9d64645f/recaptcha.go
type RecaptchaResponse struct {
	Success     bool      `json:"success"`
	ChallengeTS time.Time `json:"challenge_ts"`
	Hostname    string    `json:"hostname"`
	ErrorCodes  []int     `json:"error-codes"`
}

const recaptchaServerName = "https://www.google.com/recaptcha/api/siteverify"

// check uses the client ip address, the challenge code from the reCaptcha form,
// and the client's response input to that challenge to determine whether or not
// the client answered the reCaptcha input question correctly.
// It returns a boolean value indicating whether or not the client answered correctly.
func check(remoteip, response string) (r RecaptchaResponse, err error) {
	resp, err := http.PostForm(recaptchaServerName,
		url.Values{"secret": {"6LclI-8SAAAAADOW1hRPRm3QTJa7zXQ26V_pYFY2"}, "remoteip": {remoteip}, "response": {response}})
	if err != nil {
		log.Printf("Post error: %s\n", err)
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Read error: could not read body: %s", err)
		return
	}
	err = json.Unmarshal(body, &r)
	if err != nil {
		log.Printf("Read error: got invalid JSON: %s", err)
		return
	}
	return
}

// Confirm is the public interface function.
// It calls check, which the client ip address, the challenge code from the reCaptcha form,
// and the client's response input to that challenge to determine whether or not
// the client answered the reCaptcha input question correctly.
// It returns a boolean value indicating whether or not the client answered correctly.
func Confirm(remoteip, response string) (result bool, err error) {
	resp, err := check(remoteip, response)
	result = resp.Success
	return
}

// processCaptcha accepts the http.Request object, finds the reCaptcha form variables which
// were input and sent by HTTP POST to the server, then calls the recaptcha package's Confirm()
// method, which returns a boolean indicating whether or not the client answered the form correctly.
func processCaptcha(w http.ResponseWriter, r *http.Request) {
	recaptchaResponse, responseFound := r.Form["g-recaptcha-response"]
	if responseFound {
		result, err := Confirm(r.RemoteAddr, recaptchaResponse[0])
		if err != nil {
			http.Error(w, "No.", http.StatusServiceUnavailable)
			return
		}
		if !result {
			http.Error(w, "No.", http.StatusServiceUnavailable)
			return
		}
	}
	return
}

// HostSwitch multidomain code taken from sample code for httprouter: https://github.com/julienschmidt/httprouter
// We need an object that implements the http.Handler interface.
// Therefore we need a type for which we implement the ServeHTTP method.
// We just use a map here, in which we map host names (with port) to http.Handlers
type HostSwitch struct {
	hostMap HostMap
	theEnv  *thingEnv
}

type HostMap map[string]http.Handler

// Implement the ServerHTTP method on our new type
func (hs HostSwitch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if a http.Handler is registered for the given host.
	// If yes, use it to handle the request.
	shortregex := regexp.MustCompile("([A-Za-z0-9]+)." + viper.GetString("ShortTLD"))

	if handler := hs.hostMap[r.Host]; handler != nil {
		handler.ServeHTTP(w, r)
		// Build up subdomain matching
		// Putting the host match into the params["name"] to be retrieved later
	} else if shortregex.MatchString(r.Host) {
		name := shortregex.FindStringSubmatch(r.Host)[1]
		mymap := map[string]string{
			"name": name,
		}
		
		ctx := context.WithValue(r.Context(), httptreemux.ParamsContextKey, mymap)
		//log.Println(r.Context().Value(httptreemux.ParamsContextKey))
		hs.theEnv.shortUrlHandler(w, r.WithContext(ctx))
	} else {
		// Handle host names for which no handler is registered
		log.Println(r.Host)
		http.Error(w, "Forbidden", 403) // Or Redirect?
	}
}

//Flags
//var fLocal = flag.Bool("l", false, "Turn on localhost resolving for Handlers")

//Base struct, Page ; has to be wrapped in a data {} strut for consistency reasons
type Page struct {
	TheName  string
	Title    string
	UN       string
	IsAdmin  bool
	Token    template.HTML
	FlashMsg string
	MainTLD  string
}

type ListPage struct {
	*Page
	Pastes      []*Paste
	Files       []*File
	Shorturls   []*Shorturl
	Images      []*Image
	Screenshots []*Screenshot
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

type Screenshot struct {
	Created  int64
	Filename string
	Hits     int64
}

type Shorturl struct {
	Created int64
	Short   string
	Long    string
	Hits    int64
}

// Attempt at consolidated type:
type Thing interface{
	Type() string
	Created() int64
	Title() string
	Save() error
}

type ThingByDate []Thing
func (a ThingByDate) Len() int           { return len(a) }
func (a ThingByDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ThingByDate) Less(i, j int) bool { return a[i].Created() > a[j].Created() }

func(p Paste) Type() string {
	return "Paste"
}

func(p Paste) Save() error {
	return p.save(nil)
}

// Sorting functions
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

func init() {

	/*
			Port     string
			Email    string
			ImgDir   string
			FileDir  string
			ThumbDir string
			MainTLD  string
			ShortTLD string
			ImageTLD string
			GifTLD   string
		    AuthDB   string
		    AuthConf struct {
		        LdapEnabled bool
		        LdapConf struct {
		            LdapPort uint16 `json:",omitempty"`
		            LdapUrl  string `json:",omitempty"`
		            LdapDn   string `json:",omitempty"`
		            LdapUn   string `json:",omitempty"`
		            LdapOu   string `json:",omitempty"`
		        }
		    }
	*/

	//viper.Unmarshal(&cfg)
	//viper.UnmarshalKey("AuthConf", &auth.Authcfg)

	pflag.StringVar(&dataDir, "DataDir", "./data/", "Path to store permanent data in.")
	pflag.Parse()

	bufpool = bpool.NewBufferPool(64)
}

func markdownRender(content []byte) []byte {
	htmlFlags := 0
	htmlFlags |= blackfriday.HTML_FOOTNOTE_RETURN_LINKS
	htmlFlags |= blackfriday.HTML_TOC
	htmlFlags |= blackfriday.HTML_NOFOLLOW_LINKS
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

func isAdmin(s string) bool {
	if s == "User" {
		return false
	} else if s == "Admin" {
		return true
	}
	return false
}

//Hack to allow me to make full URLs due to absence of http:// from URL.Scheme in dev situations
//When behind Nginx, use X-Forwarded-Proto header to retrieve this, then just tack on "://"
//getScheme(r) should return http:// or https://
func getScheme(r *http.Request) (scheme string) {
	defer httputils.TimeTrack(time.Now(), "getScheme")
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

func renderTemplate(env *thingEnv, w http.ResponseWriter, name string, data interface{}) error {
	defer httputils.TimeTrack(time.Now(), "renderTemplate")
	tmpl, ok := env.templates[name]
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
	defer httputils.TimeTrack(time.Now(), "ParseBool")
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return boolValue
}

func loadPage(title string, w http.ResponseWriter, r *http.Request) (*Page, error) {
	defer httputils.TimeTrack(time.Now(), "loadPage")
	//timer.Step("loadpageFunc")
	user, isAdmin := auth.GetUsername(r.Context())
	msg := auth.GetFlash(r.Context())
	token := csrf.TemplateField(r)

	var message string
	if msg != "" {
		message = `
			<div class="alert callout" data-closable>
			<h5>Alert!</h5>
			<p>` + msg + `</p>
			<button class="close-button" aria-label="Dismiss alert" type="button" data-close>
				<span aria-hidden="true">&times;</span>
			</button>
			</div>			
        `
	} else {
		message = ""
	}

	return &Page{
		TheName: "GoThing", 
		Title: title, 
		UN: user, 
		IsAdmin: isAdmin,
		Token: token, 
		FlashMsg: message,
		MainTLD: viper.GetString("MainTLD"),
		}, nil
}

func loadMainPage(title string, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	defer httputils.TimeTrack(time.Now(), "loadMainPage")
	p, err := loadPage(title, w, r)
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

func (env *thingEnv) loadListPage(w http.ResponseWriter, r *http.Request) (*ListPage, error) {
	defer httputils.TimeTrack(time.Now(), "loadListPage")
	page, perr := loadPage("List", w, r)
	if perr != nil {
		return nil, perr
	}

	db := env.getDB()
	defer env.closeDB()

	var files []*File
	//Lets try this with boltDB now!
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Files"))
		b.ForEach(func(k, v []byte) error {
			httputils.Debugln("FILES: key=" + string(k) + " value=" + string(v))
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
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Pastes"))
		b.ForEach(func(k, v []byte) error {
			httputils.Debugln("PASTE: key=" + string(k) + " value=" + string(v))
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
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Shorturls"))
		b.ForEach(func(k, v []byte) error {
			httputils.Debugln("SHORT: key=" + string(k) + " value=" + string(v))
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
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Images"))
		b.ForEach(func(k, v []byte) error {
			httputils.Debugln("IMAGE: key=" + string(k) + " value=" + string(v))
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

	var screenshots []*Screenshot
	//Lets try this with boltDB now!
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Screenshots"))
		b.ForEach(func(k, v []byte) error {
			httputils.Debugln("SCREENSHOTS: key=" + string(k) + " value=" + string(v))
			var screenshot *Screenshot
			err := json.Unmarshal(v, &screenshot)
			if err != nil {
				log.Panicln(err)
			}
			screenshots = append(screenshots, screenshot)
			return nil
		})
		return nil
	})
	sort.Sort(ScreenshotByDate(screenshots))

	return &ListPage{Page: page, Pastes: pastes, Files: files, Shorturls: shorts, Images: images, Screenshots: screenshots}, nil
}

func ParseMultipartFormProg(r *http.Request, maxMemory int64) error {
	defer httputils.TimeTrack(time.Now(), "ParseMultipartFormProg")

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

func (f *File) save(env *thingEnv) error {
	defer httputils.TimeTrack(time.Now(), "File.save()")

	db := env.getDB()
	defer env.closeDB()

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

func (s *Shorturl) save(env *thingEnv) error {
	defer httputils.TimeTrack(time.Now(), "Shorturl.save()")

	db := env.getDB()
	defer env.closeDB()
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

func (p *Paste) save(env *thingEnv) error {
	defer httputils.TimeTrack(time.Now(), "Paste.save()")
	db := env.getDB()
	defer env.closeDB()
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
	defer httputils.TimeTrack(time.Now(), "makeThumb")
	contentType := mime.TypeByExtension(filepath.Ext(path.Base(fpath)))
	if contentType == "video/webm" {
		resize := exec.Command("/usr/bin/ffmpeg", "-i", fpath, "-vframes", "1", "-filter:v", "scale='-1:300'", thumbpath)
		err := resize.Run()
		if err != nil {
			log.Panicln(err)
		}
		return
	} else	if contentType == "video/mp4" {
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

func (i *Image) save(env *thingEnv) error {
	defer httputils.TimeTrack(time.Now(), "Image.save()")
	db := env.getDB()
	defer env.closeDB()
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

func (s *Screenshot) save(env *thingEnv) error {
	defer httputils.TimeTrack(time.Now(), "Screenshot.save()")
	db := env.getDB()
	defer env.closeDB()
	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Screenshots"))
		encoded, err := json.Marshal(s)
		if err != nil {
			log.Panicln(err)
			return err
		}
		return b.Put([]byte(s.Filename), encoded)
	})
	if err != nil {
		log.Panicln(err)
		return err
	}
	log.Println("++++Screenshot SAVED")
	return nil
}

func defaultHandler(next http.Handler) http.Handler {
	defer httputils.TimeTrack(time.Now(), "defaultHandler")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host == viper.GetString("ImageTLD") || r.Host == viper.GetString("MainTLD") || r.Host == "www."+viper.GetString("MainTLD") || r.Host == viper.GetString("ShortTLD") || r.Host == viper.GetString("GifTLD") || r.Host == "go.dev" || r.Host == "go.jba.io" {
			next.ServeHTTP(w, r)
		} else {
			log.Println("Not serving anything, because this request belongs to: " + r.Host)
			http.Error(w, http.StatusText(400), 400)
			return
		}
	})
}

func (env *thingEnv) dbInit() {
	db := env.getDB()
	defer env.closeDB()
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
		_, err = tx.CreateBucketIfNotExists([]byte("Screenshots"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
}

func tmplInit(env *thingEnv) error {

	templatesDir := "./templates/"
	layouts, err := filepath.Glob(templatesDir + "layouts/*.tmpl")
	if err != nil {
		panic(err)
	}
	includes, err := filepath.Glob(templatesDir + "includes/*.tmpl")
	if err != nil {
		panic(err)
	}

	funcMap := template.FuncMap{"prettyDate": httputils.PrettyDate, "safeHTML": httputils.SafeHTML, "imgClass": httputils.ImgClass, "imgExt": httputils.ImgExt}

	// Here we are prefacing every layout with what should be every includes/ .tmpl file
	// Ex: includes/sidebar.tmpl includes/bottom.tmpl includes/base.tmpl layouts/list.tmpl
	// **THIS IS VERY IMPORTANT TO ALLOW MY BASE TEMPLATE TO WORK**
	for _, layout := range layouts {
		files := append(includes, layout)
		//DEBUG TEMPLATE LOADING
		//httputils.Debugln(files)
		env.templates[filepath.Base(layout)] = template.Must(template.New("templates").Funcs(funcMap).ParseFiles(files...))
	}
	return nil
}

// Simple function to get the httptreemux params, setting it blank if there aren't any
func getParams(c context.Context) map[string]string {
	params, ok := c.Value(httptreemux.ParamsContextKey).(map[string]string)
	if !ok {
		params = make(map[string]string)
	}
	return params
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

	// Viper config
	viper.SetDefault("Port", "5000")
	viper.SetDefault("Email", "unused@the.moment")
	viper.SetDefault("ImgDir", "./data/up-imgs/")
	viper.SetDefault("FileDir", "./data/up-files/")
	viper.SetDefault("ThumbDir", "./data/thumbs/")
	viper.SetDefault("MainTLD", "squanch.space")
	viper.SetDefault("ShortTLD", "squanch.space")
	viper.SetDefault("ImageTLD", "i.squanch.space")
	viper.SetDefault("GifTLD", "big.squanch.space")
	viper.SetDefault("AuthDB", "./data/auth.db")
	viper.SetDefault("AdminUser", "admin")
	viper.SetDefault("AdminPass", "admin")
	viper.SetDefault("dbPath", "./data/bolt.db")
	viper.SetDefault("Dev", false)
	viper.SetDefault("Insecure", false)
	viper.SetDefault("Debug", false)

	viper.SetConfigName("conf")
	viper.SetConfigType("json")
	viper.AddConfigPath("./data/")
	if dataDir != "./data/" {
		viper.AddConfigPath(dataDir)
		viper.Set("ImgDir", filepath.Join(dataDir, "/up-imgs/"))
		viper.Set("FileDir", filepath.Join(dataDir, "/up-files/"))
		viper.Set("ThumbDir", filepath.Join(dataDir, "/thumbs/"))
		viper.Set("AuthDB", filepath.Join(dataDir, "/auth.db"))
		viper.Set("dbPath", filepath.Join(dataDir, "/bolt.db"))
		log.Println("omg", dataDir)
	}
	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		//panic(fmt.Errorf("Fatal error config file: %s \n", err))
		fmt.Println("No configuration file loaded - using defaults")
	}
	viper.SetEnvPrefix("gothing")
	viper.AutomaticEnv()

	if viper.GetBool("Debug") {
		httputils.Debug = true
	}

	/*
		// Set a static auth.HashKey and BlockKey to keep sessions after restarts:
		auth.HashKey = []byte("yyCF3ZXOneAPxOspTrmU8x9JxEP2XrZQCkJDkehrhBp6p765fiL55teT7Dt4Fbkp")
		auth.BlockKey = []byte("BqHzSVBFbpSZdvaDfy4jXf3OgA8Oe1mR")

		// Open and initialize auth database
		auth.Authdb = auth.Open("./data/auth.db")
		autherr := auth.AuthDbInit()
		if autherr != nil {
			log.Fatalln(autherr)
		}
		defer auth.Authdb.Close()
	*/

	dataDir1, err := os.Stat(dataDir)
	if os.IsNotExist(err) {
		err = os.Mkdir(dataDir, 0755)
		if err != nil {
			log.Fatalln(err)
		}
	}
	if os.IsExist(err) {
		if !dataDir1.IsDir() {
			log.Fatalln("./data/ is not a directory. This is where misc data is stored.")
		}
	}

	anAuthState, err := auth.NewAuthState(viper.GetString("AuthDB"), viper.GetString("AdminUser"))
	if err != nil {
		log.Fatalln(err)
	}

	var aThingDB *bolt.DB

	env := &thingEnv{
		Bolt:      &thingDB{aThingDB, viper.GetString("dbPath")},
		authState: anAuthState,
		templates: make(map[string]*template.Template),
	}

	err = tmplInit(env)
	if err != nil {
		log.Fatalln(err)
	}

	//Check for essential directory existence
	_, err = os.Stat(viper.GetString("ImgDir"))
	if err != nil {
		os.Mkdir(viper.GetString("ImgDir"), 0755)
	}
	_, err = os.Stat(viper.GetString("FileDir"))
	if err != nil {
		os.Mkdir(viper.GetString("FileDir"), 0755)
	}
	_, err = os.Stat(viper.GetString("ThumbDir"))
	if err != nil {
		os.Mkdir(viper.GetString("ThumbDir"), 0755)
	}

	env.dbInit()

	//std := alice.New(handlers.RecoveryHandler(), auth.UserEnvMiddle, auth.XsrfMiddle, httputils.Logger)
	std := alice.New(handlers.RecoveryHandler(), env.authState.UserEnvMiddle, csrf.Protect([]byte("c379bf3ac76ee306cf72270cf6c5a612e8351dcb")), httputils.Logger)

	if viper.GetBool("Insecure") {
		std = alice.New(handlers.RecoveryHandler(), env.authState.UserEnvMiddle, csrf.Protect([]byte("c379bf3ac76ee306cf72270cf6c5a612e8351dcb"), csrf.Secure(false)), httputils.Logger)
		//std = alice.New(handlers.ProxyHeaders, handlers.RecoveryHandler(), auth.UserEnvMiddle, auth.XsrfMiddle, httputils.Logger)
	} else {
		log.Println("Listening on " + viper.GetString("MainTLD") + " domain")
	}	

	if viper.GetBool("Dev") {
		viper.Set("MainTLD", "main.devd.io")
		viper.Set("ShortTLD", "s.devd.io")
		viper.Set("ImageTLD", "i.devd.io")
		viper.Set("GifTLD", "big.devd.io")

		log.Println("Listening on devd.io domains due to -l flag...")
		std = alice.New(handlers.ProxyHeaders, handlers.RecoveryHandler(), env.authState.UserEnvMiddle, csrf.Protect([]byte("c379bf3ac76ee306cf72270cf6c5a612e8351dcb"), csrf.Secure(false)), httputils.Logger)
		//std = alice.New(handlers.ProxyHeaders, handlers.RecoveryHandler(), auth.UserEnvMiddle, auth.XsrfMiddle, httputils.Logger)
	} else {
		log.Println("Listening on " + viper.GetString("MainTLD") + " domain")
	}

	//r := mux.NewRouter().StrictSlash(true)
	//d := r.Host(viper.GetString("MainTLD")).Subrouter()

	// Declare various routers used
	d := httptreemux.NewContextMux()
	d.PanicHandler = httptreemux.ShowErrorsPanicHandler
	i := httptreemux.NewContextMux()
	i.PanicHandler = httptreemux.ShowErrorsPanicHandler
	big := httptreemux.NewContextMux()
	big.PanicHandler = httptreemux.ShowErrorsPanicHandler
	//wild := httptreemux.New()
	//wild.PanicHandler = httptreemux.ShowErrorsPanicHandler

	log.Println("Port: " + viper.GetString("Port"))

	d.GET("/", env.indexHandler)
	d.GET("/help", env.helpHandler)
	d.GET("/priv", env.authState.AuthMiddle(env.Readme))
	d.GET("/readme", env.Readme)
	d.GET("/changelog", env.Changelog)
	d.POST("/login", env.authState.LoginPostHandler)
	d.GET("/login", env.loginPageHandler)
	d.POST("/logout", env.authState.LogoutHandler)
	d.GET("/logout", env.authState.LogoutHandler)
	//d.GET("/signup", signupPageHandler)

	//a := d.PathPrefix("/auth").Subrouter()
	a := d.NewGroup("/auth")
	a.POST("/login", env.authState.LoginPostHandler)
	a.POST("/logout", env.authState.LogoutHandler)
	a.GET("/logout", env.authState.LogoutHandler)
	a.POST("/signup", env.authState.SignupPostHandler)

	//admin := d.PathPrefix("/admin").Subrouter()
	admin := d.NewGroup("/admin")
	admin.GET("/", env.authState.AuthAdminMiddle(env.adminHandler))
	admin.POST("/users", env.authState.AuthAdminMiddle(env.authState.UserSignupPostHandler))
	//admin.POST("/user_signup", auth.AuthAdminMiddle(auth.UserSignupPostHandler))
	admin.GET("/users", env.authState.AuthAdminMiddle(env.adminSignupHandler))
	admin.GET("/list", env.authState.AuthAdminMiddle(env.adminListHandler))
	//admin.POST("/password_change", auth.AuthAdminMiddle(auth.AdminUserPassChangePostHandler))
	//admin.POST("/user_delete", auth.AuthAdminMiddle(auth.AdminUserDeletePostHandler))
	admin.POST("/user/password_change", env.authState.AuthAdminMiddle(env.authState.AdminUserPassChangePostHandler))
	admin.POST("/user/delete", env.authState.AuthAdminMiddle(env.authState.AdminUserDeletePostHandler))

	d.GET("/list", env.authState.AuthMiddle(env.listHandler))
	d.GET("/s", env.authState.AuthMiddle(env.shortenPageHandler))
	d.GET("/short", env.authState.AuthMiddle(env.shortenPageHandler))
	d.GET("/lg", env.lgHandler)
	d.GET("/p", env.pastePageHandler)
	d.GET("/p/:name", env.pasteHandler)
	d.GET("/up", env.uploadPageHandler)
	d.GET("/iup", env.uploadImagePageHandler)
	d.GET("/search/:name", env.authState.AuthMiddle(env.searchHandler))
	d.GET("/d/:name", env.downloadHandler)
	d.GET("/big/:name", imageBigHandler)
	d.GET("/i/:name", env.downloadImageHandler)
	d.GET("/md/:name", env.viewMarkdownHandler)
	d.GET("/thumbs/:name", imageThumbHandler)
	d.GET("/imagedirect/:name", imageDirectHandler)
	d.GET("/i", env.galleryHandler)
	//d.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {utils.WriteJ(w, "LOL", false)}).Methods("GET", "POST")
	//d.HandleFunc("/json2", func(w http.ResponseWriter, r *http.Request) {utils.WriteJ(w, "", false)}).Methods("GET", "POST")

	//CLI API Functions
	d.PUT("/up/*name", env.APInewFile)
	d.PUT("/up", env.APInewFile)
	d.PUT("/p/*name", env.APInewPaste)
	d.PUT("/p", env.APInewPaste)
	d.POST("/lg", env.APIlgAction)

	//API Functions
	//api := d.PathPrefix("/api").Subrouter()
	api := d.NewGroup("/api")
	api.GET("/delete/:type/:name", env.authState.AuthMiddle(env.APIdeleteHandler))
	api.POST("/paste/new", env.APInewPasteForm)
	api.POST("/file/new", env.APInewFile)
	api.POST("/file/remote", env.APInewRemoteFile)
	api.POST("/shorten/new", env.APInewShortUrlForm)
	api.POST("/lg", env.APIlgAction)
	api.POST("/image/new", env.APInewImage)
	api.POST("/image/remote", env.APInewRemoteImage)
	//Golang-Stats-API
	//api.HandleFunc("/stats", stats_api.Handler)
	//api.GET("/vars",httputils.HandleExpvars)

	//Dedicated image subdomain routes
	//i := r.Host(viper.GetString("ImageTLD")).Subrouter()
	i.GET("/", env.galleryEsgyHandler)
	i.GET("/thumbs/:name", imageThumbHandler)
	i.GET("/imagedirect/:name", imageDirectHandler)
	i.GET("/big/:name", imageBigHandler)
	i.GET("/:name", env.downloadImageHandler)

	//Big GIFs
	//big := r.Host(viper.GetString("GifTLD")).Subrouter()
	big.GET("/i/:name", imageDirectHandler)
	big.GET("/:name", imageBigHandler)

	//Dynamic subdomains | try to avoid taking www.es.gy
	//wild := r.Host("{name:([^www][A-Za-z0-9]+)}.es.gy").Subrouter()
	//wildString := "{name}."+viper.GetString("ShortTLD")
	//wild := r.Host("{name}.es.gy").Subrouter()
	//wild.GET("/", shortUrlHandler)
	//Main Short URL page
	// Collapsing this into main TLD
	//short := r.Host(viper.GetString("ShortTLD")).Subrouter()
	//short.HandleFunc("/{name}", shortUrlHandler).Methods("GET")

	//static := http.Handler(http.FileServer(http.Dir("./public/")))
	//r.PathPrefix("/").Handler(defaultHandler(static))

	//r.PathPrefix("/assets/").HandlerFunc(staticHandler)
	d.GET("/*name", env.shortUrlHandler)

	httputils.StaticInit()
	//Used for troubleshooting proxy headers
	http.HandleFunc("/omg", func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Host)
		log.Println(r.Header)
	})

	hm := make(HostMap)
	hm[viper.GetString("MainTLD")] = d
	hm[viper.GetString("ImageTLD")] = i
	hm[viper.GetString("GifTLD")] = big

	hs := &HostSwitch{
		hostMap: hm,
		theEnv:  env,
	}

	http.Handle("/", std.Then(hs))
	http.ListenAndServe("127.0.0.1:"+viper.GetString("Port"), nil)

}
