package main

// Recent changes:
// - Moved from gorilla/mux to httptreemux+go1.7 context

// TODO
// - Guard file/image upload pages from respective filetypes
// - Add a screenshot sharing route, separate from image gallery
// - Refactor all save() functions to do the actual file saving as well...
// ...only saving if the BoltDB function doesn't error out

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"git.jba.io/go/auth"
	"git.jba.io/go/httputils"
	"git.jba.io/go/thing/things"
	"git.jba.io/go/thing/vfs/assets"
	"git.jba.io/go/thing/vfs/templates"
	"github.com/boltdb/bolt"
	"github.com/disintegration/imaging"
	"github.com/ezzarghili/recaptcha-go"
	raven "github.com/getsentry/raven-go"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/oxtoacart/bpool"
	"github.com/russross/blackfriday"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	_ "github.com/tevjef/go-runtime-metrics/expvar"
)

type configuration struct {
	Port           string
	Email          string
	ImgDir         string
	FileDir        string
	ThumbDir       string
	MainTLD        string
	ShortTLD       string
	ImageTLD       string
	GifTLD         string
	CaptchaSiteKey string
	CaptchaSecret  string
}

type thingEnv struct {
	authState *auth.State
	templates map[string]*template.Template
	captcha   *recaptcha.ReCAPTCHA
}

type thingDB struct {
	*bolt.DB
	path string
}

var (
	bufpool        *bpool.BufferPool
	_24K           int64 = (1 << 20) * 24
	dataDir        string
	boltPath       string
	errNOSUCHTHING = errors.New("Thing does not exist")
	//db, _     = bolt.Open("./data/bolt.db", 0600, nil)
	//cfg       = configuration{}
)

func getDB() *bolt.DB {
	db, err := bolt.Open(boltPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Fatalln("BoltDB Error:", err)
	}
	return db
}

func imgExt(s string) string {
	ext := filepath.Ext(s)
	if ext != "" {
		ext = strings.TrimLeft(ext, ".")
	}
	return ext
}

/*
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
*/

//Base struct, Page ; has to be wrapped in a data {} strut for consistency reasons
type Page struct {
	TheName        string
	Title          string
	UN             string
	IsAdmin        bool
	Token          template.HTML
	FlashMsg       string
	MainTLD        string
	CaptchaSiteKey string
}

type ListPage struct {
	*Page
	Pastes      []*things.Paste
	Files       []*things.File
	Shorturls   []*things.Shorturl
	Images      []*things.Image
	Screenshots []*things.Screenshot
}

type GalleryPage struct {
	*Page
	Images []*things.Image
}

func init() {

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
		raven.CaptureError(err, map[string]string{"func": "renderTemplate"})
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
		raven.CaptureError(err, map[string]string{"func": "ParseBool"})
		return false
	}
	return boolValue
}

func loadPage(title string, w http.ResponseWriter, r *http.Request) (*Page, error) {
	defer httputils.TimeTrack(time.Now(), "loadPage")
	//timer.Step("loadpageFunc")
	user := auth.GetUserState(r.Context())
	msg := auth.GetFlash(r.Context())
	token := auth.CSRFTemplateField(r)

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
		TheName:        "GoThing",
		Title:          title,
		UN:             user.GetName(),
		IsAdmin:        user.IsAdmin(),
		Token:          token,
		FlashMsg:       message,
		MainTLD:        viper.GetString("MainTLD"),
		CaptchaSiteKey: viper.GetString("CaptchaSiteKey"),
	}, nil
}

func loadMainPage(title string, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	defer httputils.TimeTrack(time.Now(), "loadMainPage")
	p, err := loadPage(title, w, r)
	if err != nil {
		raven.CaptureError(err, nil)
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

	db := getDB()
	defer db.Close()

	var files []*things.File
	//Lets try this with boltDB now!
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Files"))
		err := b.ForEach(func(k, v []byte) error {
			httputils.Debugln("FILES: key=" + string(k) + " value=" + string(v))
			var file *things.File
			err := json.Unmarshal(v, &file)
			if err != nil {
				return err
			}
			files = append(files, file)
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Sort(things.FileByDate(files))

	var pastes []*things.Paste
	//Lets try this with boltDB now!
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Pastes"))
		err := b.ForEach(func(k, v []byte) error {
			httputils.Debugln("PASTE: key=" + string(k) + " value=" + string(v))
			var paste *things.Paste
			err := json.Unmarshal(v, &paste)
			if err != nil {
				return err
			}
			pastes = append(pastes, paste)
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Sort(things.PasteByDate(pastes))

	var shorts []*things.Shorturl
	//Lets try this with boltDB now!
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Shorturls"))
		err := b.ForEach(func(k, v []byte) error {
			httputils.Debugln("SHORT: key=" + string(k) + " value=" + string(v))
			var short *things.Shorturl
			err := json.Unmarshal(v, &short)
			if err != nil {
				return err
			}
			shorts = append(shorts, short)
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Sort(things.ShortByDate(shorts))

	var images []*things.Image
	//Lets try this with boltDB now!
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Images"))
		err := b.ForEach(func(k, v []byte) error {
			httputils.Debugln("IMAGE: key=" + string(k) + " value=" + string(v))
			var image *things.Image
			err := json.Unmarshal(v, &image)
			if err != nil {
				return err
			}
			images = append(images, image)
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Sort(things.ImageByDate(images))

	var screenshots []*things.Screenshot
	//Lets try this with boltDB now!
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Screenshots"))
		err := b.ForEach(func(k, v []byte) error {
			httputils.Debugln("SCREENSHOTS: key=" + string(k) + " value=" + string(v))
			var screenshot *things.Screenshot
			err := json.Unmarshal(v, &screenshot)
			if err != nil {
				return err
			}
			screenshots = append(screenshots, screenshot)
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Sort(things.ScreenshotByDate(screenshots))

	return &ListPage{Page: page, Pastes: pastes, Files: files, Shorturls: shorts, Images: images, Screenshots: screenshots}, nil
}

func ParseMultipartFormProg(r *http.Request, maxMemory int64) error {
	defer httputils.TimeTrack(time.Now(), "ParseMultipartFormProg")

	if r.Form == nil {
		err := r.ParseForm()
		if err != nil {
			raven.CaptureError(err, nil)
			return err
		}
	}
	if r.MultipartForm != nil {
		return nil
	}

	mr, err := r.MultipartReader()
	if err != nil {
		raven.CaptureError(err, nil)
		return err
	}

	f, err := mr.ReadForm(maxMemory)
	if err != nil {
		raven.CaptureError(err, nil)
		return err
	}
	for k, v := range f.Value {
		r.Form[k] = append(r.Form[k], v...)
	}
	r.MultipartForm = f

	return nil
}

func makeThumb(fpath, thumbpath string) {
	defer httputils.TimeTrack(time.Now(), "makeThumb")
	contentType := mime.TypeByExtension(filepath.Ext(path.Base(fpath)))
	if contentType == "video/webm" {
		resize := exec.Command("/usr/bin/ffmpeg", "-i", fpath, "-vframes", "1", "-filter:v", "scale='-1:300'", thumbpath)
		err := resize.Run()
		if err != nil {
			raven.CaptureErrorAndWait(err, nil)
			log.Panicln(err)
		}
		return
	} else if contentType == "video/mp4" {
		resize := exec.Command("/usr/bin/ffmpeg", "-i", fpath, "-vframes", "1", "-filter:v", "scale='-1:300'", thumbpath)
		err := resize.Run()
		if err != nil {
			raven.CaptureErrorAndWait(err, nil)
			log.Panicln(resize.Args, err)
		}
		return
	}

	img, err := imaging.Open(fpath)
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		log.Panicln(err)
		return
	}
	thumb := imaging.Fit(img, 600, 300, imaging.CatmullRom)
	err = imaging.Save(thumb, thumbpath)
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		log.Panicln(err)
		return
	}
	return
}

func defaultHandler(next http.Handler) http.Handler {
	defer httputils.TimeTrack(time.Now(), "defaultHandler")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host == viper.GetString("ImageTLD") || r.Host == viper.GetString("MainTLD") || r.Host == "www."+viper.GetString("MainTLD") || r.Host == viper.GetString("ShortTLD") || r.Host == viper.GetString("GifTLD") || r.Host == "go.dev" || r.Host == "go.jba.io" {
			next.ServeHTTP(w, r)
		} else {
			//log.Println("Not serving anything, because this request belongs to: " + r.Host)
			http.Error(w, http.StatusText(400), 400)
			return
		}
	})
}

func (env *thingEnv) dbInit() {
	db := getDB()
	defer db.Close()
	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("Pastes"))
		if err != nil {
			raven.CaptureError(err, nil)
			return fmt.Errorf("create bucket: %s", err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte("Files"))
		if err != nil {
			raven.CaptureError(err, nil)
			return fmt.Errorf("create bucket: %s", err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte("Shorturls"))
		if err != nil {
			raven.CaptureError(err, nil)
			return fmt.Errorf("create bucket: %s", err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte("Images"))
		if err != nil {
			raven.CaptureError(err, nil)
			return fmt.Errorf("create bucket: %s", err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte("SubShorturl"))
		if err != nil {
			raven.CaptureError(err, nil)
			return fmt.Errorf("create bucket: %s", err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte("Screenshots"))
		if err != nil {
			raven.CaptureError(err, nil)
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
}

func errRedir(err error, w http.ResponseWriter) {
	//log.Println(err)
	raven.CaptureError(err, nil)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

/*
// Override CSRF error handler, so it clears the CSRF cookie upon failure:
func csrfErrHandler(w http.ResponseWriter, r *http.Request) {

	cookie := &http.Cookie{
		Name:     "_gorilla_csrf",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Now().Add(-7 * 24 * time.Hour),
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
	http.Error(w, fmt.Sprintf("%s - %s",
		http.StatusText(http.StatusForbidden), csrf.FailureReason(r)),
		http.StatusForbidden)
	return
}
*/

func getThing(t things.Thing, name string) error {
	thingType := t.GetType()

	db := getDB()
	defer db.Close()

	err := db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket([]byte(thingType)).Get([]byte(name))
		//Because BoldDB's View() doesn't return an error if there's no key found, just throw a 404 on nil
		//After JSON Unmarshal, Content should be in paste.Content field
		if v == nil {
			return errNOSUCHTHING
		}
		err := json.Unmarshal(v, &t)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func saveThing(t things.Thing) error {
	name := t.Name()
	thingType := t.GetType()

	encoded, err := json.Marshal(t)
	if err != nil {
		raven.CaptureError(err, nil)
		return err
	}

	db := getDB()
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(thingType))
		return b.Put([]byte(name), encoded)
	})
	if err != nil {
		raven.CaptureError(err, nil)
		return err
	}
	//log.Println(thingType + " successfully saved!")
	return nil
}

func updateHits(t things.Thing) {
	t.UpdateHits()
	err := saveThing(t)
	if err != nil {
		raven.CaptureError(err, nil)
	}
}

func main() {

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
	viper.SetDefault("CaptchaSiteKey", "")
	viper.SetDefault("CaptchaSecret", "")
	viper.SetDefault("RavenDSN", "")

	viper.SetConfigName("gothing")
	viper.SetConfigType("json")
	viper.AddConfigPath("./data/")
	viper.AddConfigPath("/etc/")
	if dataDir != "./data/" {
		viper.AddConfigPath(dataDir)
		viper.Set("ImgDir", filepath.Join(dataDir, "/up-imgs/"))
		viper.Set("FileDir", filepath.Join(dataDir, "/up-files/"))
		viper.Set("ThumbDir", filepath.Join(dataDir, "/thumbs/"))
		viper.Set("AuthDB", filepath.Join(dataDir, "/auth.db"))
		viper.Set("dbPath", filepath.Join(dataDir, "/bolt.db"))
	}
	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		log.Println("Error loading configuration:", err)
	}
	viper.SetEnvPrefix("gothing")
	viper.AutomaticEnv()

	if viper.GetBool("Debug") {
		httputils.Debug = true
	}

	// Set boltDB path as a global var for easy access
	boltPath = viper.GetString("dbPath")

	raven.SetDSN(viper.GetString("RavenDSN"))

	dataDir1, err := os.Stat(dataDir)
	if os.IsNotExist(err) {
		err = os.Mkdir(dataDir, 0755)
		if err != nil {
			raven.CaptureErrorAndWait(err, nil)
			log.Fatalln(err)
		}
	}
	if os.IsExist(err) {
		if !dataDir1.IsDir() {
			log.Fatalln("./data/ is not a directory. This is where misc data is stored.")
		}
	}

	theCaptcha, err := recaptcha.NewReCAPTCHA(viper.GetString("CaptchaSecret"))
	if err != nil {
		log.Fatalln("Error initializing recaptcha instance:", err)
	}

	env := &thingEnv{
		authState: auth.NewAuthState(viper.GetString("AuthDB")),
		templates: make(map[string]*template.Template),
		captcha:   &theCaptcha,
	}

	env.templates = templates.TmplInit()

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

	r := mux.NewRouter().StrictSlash(false)

	if viper.GetBool("Dev") {
		viper.Set("MainTLD", "localhost")
		viper.Set("ShortTLD", "s.localhost")
		viper.Set("ImageTLD", "i.localhost")
		viper.Set("GifTLD", "big.localhost")

		log.Println("Listening on localhost domains due to -l flag...")
		r.Use(env.authState.CSRFProtect(false))
		//std = alice.New(handlers.ProxyHeaders, handlers.RecoveryHandler(), env.authState.UserEnvMiddle, csrf.Protect([]byte("c379bf3ac76ee306cf72270cf6c5a612e8351dcb"), csrf.Secure(false)), httputils.Logger)
		//std = alice.New(handlers.ProxyHeaders, handlers.RecoveryHandler(), auth.UserEnvMiddle, auth.XsrfMiddle, httputils.Logger)
	} else {
		log.Println("Listening on " + viper.GetString("MainTLD") + " domain")
		r.Use(env.authState.CSRFProtect(true))
	}

	//r.Use(handlers.ProxyHeaders)
	r.Use(handlers.RecoveryHandler())
	r.Use(env.authState.UserEnvMiddle)

	r.Use(httputils.Logger)
	d := r.Host(viper.GetString("MainTLD")).Subrouter()

	// Declare various routers used
	//i := r.Host(viper.GetString("ImageTLD")).Subrouter()
	//big := r.Host(viper.GetString("GifTLD")).Subrouter()

	log.Println("Port: " + viper.GetString("Port"))

	d.HandleFunc("/", env.indexHandler).Methods("GET")
	d.HandleFunc("/index", env.indexHandler).Methods("GET")
	d.HandleFunc("/help", env.helpHandler).Methods("GET")
	d.HandleFunc("/priv", env.authState.AuthMiddle(env.Readme)).Methods("GET")
	d.HandleFunc("/readme", env.Readme).Methods("GET")
	d.HandleFunc("/changelog", env.Changelog).Methods("GET")
	d.HandleFunc("/login", env.authState.LoginPostHandler).Methods("POST")
	d.HandleFunc("/login", env.loginPageHandler).Methods("GET")
	d.HandleFunc("/logout", env.authState.LogoutHandler).Methods("POST")
	d.HandleFunc("/logout", env.authState.LogoutHandler).Methods("GET")
	d.HandleFunc("/signup", env.signupPageHandler).Methods("GET")

	a := d.PathPrefix("/auth").Subrouter()
	//a := d.NewGroup("/auth")
	a.HandleFunc("/login", env.authState.LoginPostHandler).Methods("POST")
	a.HandleFunc("/logout", env.authState.LogoutHandler).Methods("POST")
	a.HandleFunc("/logout", env.authState.LogoutHandler).Methods("GET")
	a.HandleFunc("/signup", env.authState.UserSignupPostHandler).Methods("POST")

	admin := d.PathPrefix("/admin").Subrouter()
	//admin := d.NewGroup("/admin")
	admin.HandleFunc("/", env.authState.AuthAdminMiddle(env.adminHandler)).Methods("GET")
	admin.HandleFunc("/users", env.authState.AuthAdminMiddle(env.adminSignupHandler)).Methods("GET")
	admin.HandleFunc("/list", env.authState.AuthAdminMiddle(env.adminListHandler)).Methods("GET")

	d.HandleFunc("/list", env.authState.AuthMiddle(env.listHandler)).Methods("GET")
	d.HandleFunc("/s", env.authState.AuthMiddle(env.shortenPageHandler)).Methods("GET")
	d.HandleFunc("/short", env.authState.AuthMiddle(env.shortenPageHandler)).Methods("GET")
	d.HandleFunc("/lg", env.lgHandler).Methods("GET")
	d.HandleFunc("/p", env.pastePageHandler).Methods("GET")
	d.HandleFunc("/p/{name}", env.pasteHandler).Methods("GET")
	d.HandleFunc("/up", env.uploadPageHandler).Methods("GET")
	d.HandleFunc("/iup", env.uploadImagePageHandler).Methods("GET")
	d.HandleFunc("/search/{name}", env.authState.AuthMiddle(env.searchHandler)).Methods("GET")
	d.HandleFunc("/d/{name}", env.downloadHandler).Methods("GET")
	d.HandleFunc("/big/{name}", imageBigHandler).Methods("GET")
	d.HandleFunc("/i/{name}", env.downloadImageHandler).Methods("GET")
	d.HandleFunc("/md/{name}", env.viewMarkdownHandler).Methods("GET")
	d.HandleFunc("/thumbs/{name}", imageThumbHandler).Methods("GET")
	d.HandleFunc("/imagedirect/{name}", imageDirectHandler).Methods("GET")
	d.HandleFunc("/i", env.galleryHandler).Methods("GET")

	//CLI API Functions
	d.HandleFunc("/up/{name:.+}", env.APInewFile).Methods("PUT")
	d.HandleFunc("/up", env.APInewFile).Methods("PUT")
	d.HandleFunc("/p/{name:.+}", env.APInewPaste).Methods("PUT")
	d.HandleFunc("/p", env.APInewPaste).Methods("PUT")
	d.HandleFunc("/lg", env.APIlgAction).Methods("POST")

	//API Functions
	api := d.PathPrefix("/api").Subrouter()
	//api := d.NewGroup("/api")
	api.HandleFunc("/delete/{type}/{name:.+}", env.authState.AuthMiddle(env.APIdeleteHandler)).Methods("GET")
	api.HandleFunc("/paste/new", env.APInewPasteForm).Methods("POST")
	api.HandleFunc("/file/new", env.APInewFile).Methods("POST")
	api.HandleFunc("/file/remote", env.APInewRemoteFile).Methods("POST")
	api.HandleFunc("/shorten/new", env.APInewShortUrlForm).Methods("POST")
	api.HandleFunc("/lg", env.APIlgAction).Methods("POST")
	api.HandleFunc("/image/new", env.APInewImage).Methods("POST")
	api.HandleFunc("/image/remote", env.APInewRemoteImage).Methods("POST")
	//Golang-Stats-API
	//api.HandleFunc("/stats", stats_api.Handler)
	//api.GET("/vars",httputils.HandleExpvars)

	//Dedicated image subdomain routes
	i := r.Host(viper.GetString("ImageTLD")).Subrouter()
	i.HandleFunc("/", env.galleryEsgyHandler).Methods("GET")
	i.HandleFunc("/thumbs/{name}", imageThumbHandler).Methods("GET")
	i.HandleFunc("/imagedirect/{name}", imageDirectHandler).Methods("GET")
	i.HandleFunc("/big/{name}", imageBigHandler).Methods("GET")
	i.HandleFunc("/{name}", env.downloadImageHandler).Methods("GET")

	//Big GIFs
	big := r.Host(viper.GetString("GifTLD")).Subrouter()
	big.HandleFunc("/i/{name}", imageDirectHandler).Methods("GET")
	big.HandleFunc("/{name}", imageBigHandler).Methods("GET")

	//Dynamic subdomains | try to avoid taking www.es.gy
	//wild := r.Host("{name:([^www][A-Za-z0-9]+)}.es.gy").Subrouter()
	//wildString := "{name}."+viper.GetString("ShortTLD")
	wild := r.Host("{name}." + viper.GetString("ShortTLD")).Subrouter()
	wild.HandleFunc("/", env.shortUrlHandler).Methods("GET")
	//Main Short URL page
	// Collapsing this into main TLD
	short := r.Host(viper.GetString("ShortTLD")).Subrouter()
	short.HandleFunc("/{name}", env.shortUrlHandler).Methods("GET")

	//static := http.Handler(http.FileServer(http.Dir("./public/")))
	//r.PathPrefix("/").Handler(defaultHandler(static))

	//r.PathPrefix("/assets/").HandlerFunc(staticHandler)
	//d.GET("/*name", env.shortUrlHandler)

	r.HandleFunc("/robots.txt", assets.Robots).Methods("GET")
	r.HandleFunc("/favicon.ico", assets.FaviconICO).Methods("GET")
	r.HandleFunc("/favicon.png", assets.FaviconPNG).Methods("GET")
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(assets.Assets)))

	//httputils.StaticInit()
	//r.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets"))))

	http.Handle("/", r)
	http.ListenAndServe("127.0.0.1:"+viper.GetString("Port"), nil)

}
