package main

// Recent changes:
// - Moved from gorilla/mux to httptreemux+go1.7 context

// TODO
// - Guard file/image upload pages from respective filetypes
// - Add a screenshot sharing route, separate from image gallery
// - Refactor all save() functions to do the actual file saving as well...
// ...only saving if the BoltDB function doesn't error out

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/GeertJohan/go.rice"
	"github.com/boltdb/bolt"
	"github.com/dimfeld/httptreemux"
	"github.com/disintegration/imaging"
	"github.com/gorilla/handlers"
	"github.com/justinas/alice"
	"github.com/oxtoacart/bpool"
	//"github.com/aqtrans/ctx-csrf"
	"html/template"
	"regexp"

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

var (
	bufpool   *bpool.BufferPool
	templates map[string]*template.Template
	_24K      int64 = (1 << 20) * 24
	fLocal    bool
	debug     bool
	//db, _     = bolt.Open("./data/bolt.db", 0600, nil)
	db *bolt.DB
	//cfg       = configuration{}
)

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
type HostSwitch map[string]http.Handler

// Implement the ServerHTTP method on our new type
func (hs HostSwitch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if a http.Handler is registered for the given host.
	// If yes, use it to handle the request.
	shortregex := regexp.MustCompile("([A-Za-z0-9]+)." + viper.GetString("ShortTLD"))

	if handler := hs[r.Host]; handler != nil {
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
		shortUrlHandler(w, r.WithContext(ctx))
	} else {
		// Handle host names for wich no handler is registered
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
	FullURL string
	Long    string
	Hits    int64
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

	//Flag '-l' enables go.dev and *.dev domain resolution
	flag.BoolVar(&fLocal, "l", false, "Turn on localhost resolving for Handlers")
	//Flag '-d' enabled debug logging
	flag.BoolVar(&httputils.Debug, "d", false, "Enabled debug logging")

	bufpool = bpool.NewBufferPool(64)
	if templates == nil {
		templates = make(map[string]*template.Template)
	}
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

func setFlash(msg string, w http.ResponseWriter, r *http.Request) {
	auth.SetSession("flash", msg, w, r)
}

func renderTemplate(w http.ResponseWriter, name string, data interface{}) error {
	defer httputils.TimeTrack(time.Now(), "renderTemplate")
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
	//token := auth.GetToken(r.Context())
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

	return &Page{TheName: "GoThing", Title: title, UN: user, IsAdmin: isAdmin, Token: token, FlashMsg: message}, nil
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

func loadListPage(w http.ResponseWriter, r *http.Request) (*ListPage, error) {
	defer httputils.TimeTrack(time.Now(), "loadListPage")
	page, perr := loadPage("List", w, r)
	if perr != nil {
		return nil, perr
	}

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

func (f *File) save() error {
	defer httputils.TimeTrack(time.Now(), "File.save()")
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

func (s *Shorturl) save() error {
	defer httputils.TimeTrack(time.Now(), "Shorturl.save()")
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

func (p *Paste) save() error {
	defer httputils.TimeTrack(time.Now(), "Paste.save()")
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
	defer httputils.TimeTrack(time.Now(), "Image.save()")
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

func (s *Screenshot) save() error {
	defer httputils.TimeTrack(time.Now(), "Screenshot.save()")
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

func Open(path string) *bolt.DB {
	var err error
	db, err = bolt.Open(path, 0600, nil)
	if err != nil {
		log.Println(err)
	}
	return db
}

func dbInit() {
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

func riceInit() error {
	// Parent templates directory named 'templates'
	templateBox, err := rice.FindBox("templates")
	if err != nil {
		return err
	}
	// Child directory 'templates/includes' containing the base templates
	includes, err := templateBox.Open("includes")
	if err != nil {
		return err
	}
	includeDir, err := includes.Readdir(-1)
	if err != nil {
		return err
	}
	// Child directory 'templates/layouts' containing individual page layouts
	layouts, err := templateBox.Open("layouts")
	if err != nil {
		return err
	}
	layoutsDir, err := layouts.Readdir(-1)
	if err != nil {
		return err
	}
	var boxT []string
	var templateIBuff bytes.Buffer
	for _, v := range includeDir {
		boxT = append(boxT, "includes/"+v.Name())
		iString, _ := templateBox.String("includes/" + v.Name())
		templateIBuff.WriteString(iString)
	}

	funcMap := template.FuncMap{"prettyDate": httputils.PrettyDate, "safeHTML": httputils.SafeHTML, "imgClass": httputils.ImgClass, "imgExt": httputils.ImgExt}

	// Here we are prefacing every layout with what should be every includes/ .tmpl file
	// Ex: includes/sidebar.tmpl includes/bottom.tmpl includes/base.tmpl layouts/list.tmpl
	// **THIS IS VERY IMPORTANT TO ALLOW MY BASE TEMPLATE TO WORK**
	for _, layout := range layoutsDir {
		boxT = append(boxT, "layouts/"+layout.Name())
		//DEBUG TEMPLATE LOADING
		//utils.Debugln(files)
		lString, _ := templateBox.String("layouts/" + layout.Name())
		fstring := templateIBuff.String() + lString
		templates[layout.Name()] = template.Must(template.New(layout.Name()).Funcs(funcMap).Parse(fstring))
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
	viper.SetDefault("Port", "3000")
	viper.SetDefault("Email", "unused@the.moment")
	viper.SetDefault("ImgDir", "./data/up-imgs/")
	viper.SetDefault("FileDir", "./data/up-files/")
	viper.SetDefault("ThumbDir", "./data/thumbs/")
	viper.SetDefault("MainTLD", "es.gy")
	viper.SetDefault("ShortTLD", "es.gy")
	viper.SetDefault("ImageTLD", "i.es.gy")
	viper.SetDefault("GifTLD", "big.es.gy")
	viper.SetDefault("AuthDB", "./data/auth.db")
	viper.SetDefault("AdminUser", "admin")

	viper.SetConfigName("conf")
	viper.AddConfigPath("./data/")
	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		//panic(fmt.Errorf("Fatal error config file: %s \n", err))
		fmt.Println("No configuration file loaded - using defaults")
	}
	viper.SetConfigType("json")
	viper.WatchConfig()

	auth.AdminUser = viper.GetString("AdminUser")
	auth.AdminPass = viper.GetString("AdminPass")
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

	httputils.AssetsBox = rice.MustFindBox("assets")

	err = riceInit()
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

	//var db, _ = bolt.Open("./bolt.db", 0600, nil)
	db = Open("./data/bolt.db")
	defer db.Close()
	dbInit()

	flag.Parse()
	flag.Set("bind", ":3000")

	//std := alice.New(handlers.RecoveryHandler(), auth.UserEnvMiddle, auth.XsrfMiddle, httputils.Logger)
	std := alice.New(handlers.RecoveryHandler(), auth.UserEnvMiddle, csrf.Protect([]byte("c379bf3ac76ee306cf72270cf6c5a612e8351dcb")), httputils.Logger)

	if fLocal {
		viper.Set("MainTLD", "main.devd.io")
		viper.Set("ShortTLD", "devd.io")
		viper.Set("ImageTLD", "i.devd.io")
		viper.Set("GifTLD", "big.devd.io")

		log.Println("Listening on devd.io domains due to -l flag...")
		std = alice.New(handlers.ProxyHeaders, handlers.RecoveryHandler(), auth.UserEnvMiddle, csrf.Protect([]byte("c379bf3ac76ee306cf72270cf6c5a612e8351dcb"), csrf.Secure(false)), httputils.Logger)
		//std = alice.New(handlers.ProxyHeaders, handlers.RecoveryHandler(), auth.UserEnvMiddle, auth.XsrfMiddle, httputils.Logger)
	} else {
		log.Println("Listening on " + viper.GetString("MainTLD") + " domain")
	}

	//r := mux.NewRouter().StrictSlash(true)
	//d := r.Host(viper.GetString("MainTLD")).Subrouter()

	// Declare various routers used
	d := httptreemux.New()
	d.PanicHandler = httptreemux.ShowErrorsPanicHandler
	i := httptreemux.New()
	i.PanicHandler = httptreemux.ShowErrorsPanicHandler
	big := httptreemux.New()
	big.PanicHandler = httptreemux.ShowErrorsPanicHandler
	//wild := httptreemux.New()
	//wild.PanicHandler = httptreemux.ShowErrorsPanicHandler

	log.Println("Port: " + viper.GetString("Port"))

	d.GET("/", indexHandler)
	d.GET("/help", helpHandler)
	d.GET("/priv", auth.AuthMiddle(Readme))
	d.GET("/readme", Readme)
	d.GET("/changelog", Changelog)
	d.POST("/login", auth.LoginPostHandler)
	d.GET("/login", loginPageHandler)
	d.POST("/logout", auth.LogoutHandler)
	d.GET("/logout", auth.LogoutHandler)
	//d.GET("/signup", signupPageHandler)

	//a := d.PathPrefix("/auth").Subrouter()
	a := d.NewGroup("/auth")
	a.POST("/login", auth.LoginPostHandler)
	a.POST("/logout", auth.LogoutHandler)
	a.GET("/logout", auth.LogoutHandler)
	a.POST("/signup", auth.SignupPostHandler)

	//admin := d.PathPrefix("/admin").Subrouter()
	admin := d.NewGroup("/admin")
	admin.GET("/", auth.AuthAdminMiddle(adminHandler))
	admin.POST("/users", auth.AuthAdminMiddle(auth.UserSignupPostHandler))
	//admin.POST("/user_signup", auth.AuthAdminMiddle(auth.UserSignupPostHandler))
	admin.GET("/users", auth.AuthAdminMiddle(adminSignupHandler))
	admin.GET("/list", auth.AuthAdminMiddle(adminListHandler))
	//admin.POST("/password_change", auth.AuthAdminMiddle(auth.AdminUserPassChangePostHandler))
	//admin.POST("/user_delete", auth.AuthAdminMiddle(auth.AdminUserDeletePostHandler))
	admin.POST("/user/password_change", auth.AuthAdminMiddle(auth.AdminUserPassChangePostHandler))
	admin.POST("/user/delete", auth.AuthAdminMiddle(auth.AdminUserDeletePostHandler))

	d.GET("/list", auth.AuthMiddle(listHandler))
	d.GET("/s", auth.AuthMiddle(shortenPageHandler))
	d.GET("/short", auth.AuthMiddle(shortenPageHandler))
	d.GET("/lg", lgHandler)
	d.GET("/p", pastePageHandler)
	d.GET("/p/:name", pasteHandler)
	d.GET("/up", uploadPageHandler)
	d.GET("/iup", uploadImagePageHandler)
	d.GET("/search/:name", auth.AuthMiddle(searchHandler))
	d.GET("/d/:name", downloadHandler)
	d.GET("/big/:name", imageBigHandler)
	d.GET("/i/:name", downloadImageHandler)
	d.GET("/md/:name", viewMarkdownHandler)
	d.GET("/thumbs/:name", imageThumbHandler)
	d.GET("/imagedirect/:name", imageDirectHandler)
	d.GET("/i", galleryHandler)
	//d.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {utils.WriteJ(w, "LOL", false)}).Methods("GET", "POST")
	//d.HandleFunc("/json2", func(w http.ResponseWriter, r *http.Request) {utils.WriteJ(w, "", false)}).Methods("GET", "POST")

	//CLI API Functions
	d.PUT("/up/*name", APInewFile)
	d.PUT("/up", APInewFile)
	d.PUT("/p/*name", APInewPaste)
	d.PUT("/p", APInewPaste)
	d.POST("/lg", APIlgAction)

	//API Functions
	//api := d.PathPrefix("/api").Subrouter()
	api := d.NewGroup("/api")
	api.GET("/delete/:type/:name", auth.AuthMiddle(APIdeleteHandler))
	api.POST("/paste/new", APInewPasteForm)
	api.POST("/file/new", APInewFile)
	api.POST("/file/remote", APInewRemoteFile)
	api.POST("/shorten/new", APInewShortUrlForm)
	api.POST("/lg", APIlgAction)
	api.POST("/image/new", APInewImage)
	api.POST("/image/remote", APInewRemoteImage)
	//Golang-Stats-API
	//api.HandleFunc("/stats", stats_api.Handler)
	//api.GET("/vars",httputils.HandleExpvars)

	//Dedicated image subdomain routes
	//i := r.Host(viper.GetString("ImageTLD")).Subrouter()
	i.GET("/", galleryEsgyHandler)
	i.GET("/thumbs/:name", imageThumbHandler)
	i.GET("/imagedirect/:name", imageDirectHandler)
	i.GET("/big/:name", imageBigHandler)
	i.GET("/:name", downloadImageHandler)

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
	d.GET("/*name", shortUrlHandler)
	http.HandleFunc("/robots.txt", httputils.RobotsHandler)
	http.HandleFunc("/favicon.ico", httputils.FaviconHandler)
	http.HandleFunc("/favicon.png", httputils.FaviconHandler)
	http.HandleFunc("/assets/", httputils.StaticHandler)
	//Used for troubleshooting proxy headers
	http.HandleFunc("/omg", func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Host)
		log.Println(r.Header)
	})

	hs := make(HostSwitch)
	hs[viper.GetString("MainTLD")] = d
	hs[viper.GetString("ImageTLD")] = i
	hs[viper.GetString("GifTLD")] = big

	http.Handle("/", std.Then(hs))
	http.ListenAndServe("127.0.0.1:"+viper.GetString("Port"), nil)

}
