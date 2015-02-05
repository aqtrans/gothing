package main

// TODO
// - separate image handling, with dropdown folder selection on upload page, and dedicated list page
// - finish replacing wiki functions with snippet and boltdb functions

import (
	"crypto/rand"
	"errors"
	"flag"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/oxtoacart/bpool"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday"
	"github.com/kennygrant/sanitize"
	"github.com/apexskier/httpauth"
	"github.com/codegangsta/negroni"
	"golang.org/x/crypto/bcrypt"
	"github.com/boltdb/bolt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"os/exec"
	"net/http"
	"net/url"
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

const timestamp = "2006-01-02_at_03:04:05PM"

var (
    backend httpauth.GobFileAuthBackend
    aaa httpauth.Authorizer
    roles map[string]httpauth.Role
    backendfile = "./data/auth.gob"
    bufpool *bpool.BufferPool
    templates map[string]*template.Template
    myUn string = "***REMOVED***"
    myURL string = "http://localhost:3000"
    myPw string = "***REMOVED***"
    myEmail string = "me@jba.io"
    _24K int64 = (1 << 20) * 24
	fLocal bool
)

var Db, _ = bolt.Open("./data/bolt.db", 0600, nil)

//Flags
//var fLocal = flag.Bool("l", false, "Turn on localhost resolving for Handlers")

//Base struct, Page ; has to be wrapped in a data {} strut for consistency reasons
type Page struct {
    Title   string
    UN      string
}

type ListPage struct {
    *Page
    Snips   []Snip
    Pastes  []Paste
    Files   []File
    Shorturls []Shorturl
}

//BoltDB structs:
type Paste struct {
	Created string
	Title string
	Content string
	Hits	int64
}

type Snip struct {
	Created string
	Title string
	Cats string
	Content []string
	Hits	int64
}

type File struct {
	Created string
	Filename string
	Hits	int64
}

type Shorturl struct {
	Created string
	Short 	string
	Long 	string
	Hits 	int64
}

func init() {
	flag.BoolVar(&fLocal, "l", false, "Turn on localhost resolving for Handlers")
	bufpool = bpool.NewBufferPool(64)
	if templates == nil {
		templates = make(map[string]*template.Template)
	}
	templatesDir := "./data/tmpl/"
	layouts, err := filepath.Glob(templatesDir + "layouts/*.tmpl")
	if err != nil {
		log.Fatal(err)
	}
	includes, err := filepath.Glob(templatesDir + "includes/*.tmpl")
	if err != nil {
		log.Fatal(err)
	}
	for _, layout := range layouts {
		files := append(includes, layout)
		//DEBUG TEMPLATE LOADING log.Println(files)
		templates[filepath.Base(layout)] = template.Must(template.ParseFiles(files...))
	}
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

func getUsername(w http.ResponseWriter, r *http.Request) (username string) {
	//defer timeTrack(time.Now(), "getUsername")
	username = ""
	user, err := aaa.CurrentUser(w, r)
	if err == nil {
        username = user.Username
	}
	return username
}

func addUser(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "addUser")
	var user httpauth.UserData
	user.Username = template.HTMLEscapeString(r.PostFormValue("username"))
	user.Email = template.HTMLEscapeString(r.PostFormValue("email"))
	password := template.HTMLEscapeString(r.PostFormValue("password"))
	user.Role = template.HTMLEscapeString(r.PostFormValue("role"))
	if err := aaa.Register(w, r, user, password); err != nil {
		log.Println(err)
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "loginHandler")
	username := template.HTMLEscapeString(r.FormValue("username"))
	password := template.HTMLEscapeString(r.FormValue("password"))
	err := aaa.Login(w, r, username, password, r.Referer())
	if err == nil {
		log.Println(username + " successfully logged in.")
		messages := aaa.Messages(w, r)
		p, err := loadPage("Successfully Logged In", username)
		data := struct {
    		Page *Page
		    Title string
		    UN string
		    Msg []string
		} {
    		p,
    		"Successfully Logged In",
    		username,
    		messages,
		}
		err = renderTemplate(w, "login.tmpl", data)
		if err != nil {
		    log.Println(err)
		    return
		}
	} else if err != nil && err.Error() == "httpauth: already authenticated" {
		log.Println(username + " already logged in.")
		messages := aaa.Messages(w, r)
		p, err := loadPage("Already Logged In", username)
		data := struct {
    		Page *Page
		    Title string
		    UN string
		    Msg []string
		} {
    		p,
    		"Already Logged In",
    		username,
    		messages,
		}
		err = renderTemplate(w, "login.tmpl", data)
		if err != nil {
		    log.Println(err)
		    return
		}
	} else {
		log.Println("LOGINHANDLER ERROR:")
		log.Println(err)
		messages := aaa.Messages(w, r)
		p, err := loadPage("Login Error", "")
		data := struct {
    		Page *Page
		    Title string
		    UN string
		    Msg []string
		} {
    		p,
    		"Login Error",
    		"",
    		messages,
		}
		err = renderTemplate(w, "login.tmpl", data)
		if err != nil {
		    log.Println(err)
		    return
		}

	}
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "logoutHandler")
	username := getUsername(w, r)
	err := aaa.Logout(w, r)
	if err != nil {
		fmt.Println(err)
		return
	}
	log.Println("Logout")
	messages := aaa.Messages(w, r)
	p, err := loadPage("Logged out", username)
	data := struct {
		Page *Page
	    Title string
	    UN string
	    Msg []string
	} {
		p,
		"Logged out",
		username,
		messages,
	}
	err = renderTemplate(w, "login.tmpl", data)
	if err != nil {
	    log.Println(err)
	    return
	}
}

func GuardPath(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := aaa.Authorize(w, r, true)
		if err != nil {
			fmt.Println(err)
			messages := aaa.Messages(w, r)
			p, err := loadPage("Please log in", "")
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
			p, err := loadPage("Please log in", "")
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

func indexHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "indexHandler")
	username := getUsername(w, r)
	//fmt.Fprintf(w, indexPage)
	title := "index"
	p, _ := loadMainPage(title, username)
	err := renderTemplate(w, "index.tmpl", p)
	if err != nil {
		log.Println(err)
	}
}

func lgHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "lgHandler")
	username := getUsername(w, r)
	//fmt.Fprintf(w, indexPage)
	title := "lg"
	p, err := loadPage(title, username)
	data := struct {
		Page *Page
	    Title string
	    UN string
	    Message string
	} {
		p,
		title,
		username,
		"",
	}
	err = renderTemplate(w, "lg.tmpl", data)
	if err != nil {
		log.Println(err)
	}
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "searchHandler")
	vars := mux.Vars(r)
	term := vars["term"]
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
    		//ptime := paste.Created.Format(timestamp)
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
    		//ptime := paste.Created.Format(timestamp)
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

func uploadPageHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "uploadPageHandler")
	username := getUsername(w, r)
	//fmt.Fprintf(w, indexPage)
	title := "up"
	p, _ := loadMainPage(title, username)
	err := renderTemplate(w, "up.tmpl", p)
	if err != nil {
		log.Println(err)
	}
}

func pastePageHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "pastePageHandler")
	username := getUsername(w, r)
	//fmt.Fprintf(w, indexPage)
	title := "paste"
	p, _ := loadMainPage(title, username)
	err := renderTemplate(w, "paste.tmpl", p)
	r.ParseForm()
	//log.Println(r.Form)
	if err != nil {
		log.Println(err)
	}
}

func shortenPageHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "shortenPageHandler")
	username := getUsername(w, r)
	//fmt.Fprintf(w, indexPage)
	title := "shorten"
	p, _ := loadMainPage(title, username)
	err := renderTemplate(w, "shorten.tmpl", p)
	r.ParseForm()
	//log.Println(r.Form)
	if err != nil {
		log.Println(err)
	}
}

func loginPageHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "loginPageHandler")
	username := getUsername(w, r)
	title := "login"
	//p, _ := loadPage(title, username)
	messages := aaa.Messages(w, r)
	p, err := loadPage(title, username)
	data := struct {
		Page *Page
	    Title string
	    UN string
	    Msg []string
	} {
		p,
		title,
		username,
		messages,
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

func rawSnipHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "rawSnipHandler")
	vars := mux.Vars(r)
	//username := getUsername(w, r)
	title := vars["page"]
	snip := &Snip{}
	//p, err := loadPage(title, username)
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
			fmt.Fprintf(w, "%s", strings.Trim(fmt.Sprint(snip.Content), "[]"))
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

func privHandler(w http.ResponseWriter, r *http.Request) {
	err := aaa.Authorize(w, r, true)
	if err != nil {
		fmt.Println(err)
		//http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	user, err := aaa.CurrentUser(w, r)
	username := getUsername(w, r)
	if err == nil {
		p, err := loadPage("Please Login", username)
		data := struct {
    		Page *Page
		    User httpauth.UserData
		} {
    		p,
    		user,
		}
		t, err := template.New("priv").Parse(`
            <html>
            <head><title>Secret page</title></head>
            <body>
                <h1>Httpauth example<h1>
                {{ with .User }}
                    <h2>Hello {{ .Username }}</h2>
                    <p>Your role is '{{ .Role }}'. Your email is {{ .Email }}.</p>
                    <p>{{ if .Role | eq "admin" }}<a href="/admin">Admin page</a> {{ end }}<a href="/logout">Logout</a></p>
                {{ end }}
                <form action="/change" method="post" id="change">
                    <h3>Change email</h3>
                    <p><input type="email" name="new_email" placeholder="new email"></p>
                    <button type="submit">Submit</button>
                </form>
            </body>
            `)
        if err != nil {
        	panic(err)
        }
        t.Execute(w, data)
	}
}

func loadPage(title, user string) (*Page, error) {
	//timer.Step("loadpageFunc")
	return &Page{Title: title, UN: user}, nil
}

func loadMainPage(title, user string) (interface{}, error) {
	//timer.Step("loadpageFunc")
	//p := &Page{Title: title, UN: user}
	p, err := loadPage(title, user)
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

func loadListPage(user string) (*ListPage, error) {
    page, perr := loadPage("List", user)
    if perr != nil {
        log.Println(perr)
    }

	/*pfiles, _ := ioutil.ReadDir("./data/paste")
	wfiles, _ := ioutil.ReadDir("./data/wiki")
	files, _ := ioutil.ReadDir("./data/uploads")

	//List of stuff
	//pl := []string{}
	wl := []string{}
	fl := []string{}
	//List of stuffs info
	pi := []string{}
	wi := []string{}
	fi := []string{}
	*/

	//layout := "Jan 2, 2006 at 3:04pm (CST)"
	/*
	for _, f := range files {
		flink := string(f.Name())
		ftime := f.ModTime().String()
		fsize := strconv.FormatInt(f.Size(), 8)
		fl = append(fl, flink)
		fi = append(fi, ftime, fsize)
	}
	for _, w := range wfiles {
		if w.IsDir() {
			sd, _ := ioutil.ReadDir("./data/wiki/" + w.Name())
			for _, wsub := range sd {
				wlink := w.Name() + "/" + strings.TrimSuffix(wsub.Name(), myExt)
				wtime := wsub.ModTime().String()
				wsize := strconv.FormatInt(wsub.Size(), 8)
				wl = append(wl, wlink)
				wi = append(wi, wtime, wsize)
			}
		} else {
		wlink := strings.TrimSuffix(w.Name(), myExt)
		wtime := w.ModTime().String()
		wsize := strconv.FormatInt(w.Size(), 8)
		wl = append(wl, wlink)
		wi = append(wi, wtime, wsize)
		}
	}*/

	snip := &Snip{}
	var snips []Snip
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
    		//ptime := paste.Created.Format(timestamp)
    		shits := snip.Hits
    		//pl = append(pl, plink)
    		//pi = append(pi, ptime, string(phits))
    		snips = []Snip{
    			Snip{
    			Created: snip.Created,
    			Title: slink,
    			Hits: shits,
    			},
    		}
	        return nil
	    })
	    return nil
	})


	file := &File{}
	var files []File
	//Lets try this with boltDB now!
	Db.View(func(tx *bolt.Tx) error {
	    b := tx.Bucket([]byte("Files"))
	    b.ForEach(func(k, v []byte) error {
	        //fmt.Printf("key=%s, value=%s\n", k, v)
	        err := json.Unmarshal(v, &file)
    		if err != nil {
    			log.Println(err)
    		}
    		flink := file.Filename
    		//ptime := paste.Created.Format(timestamp)
    		fhits := file.Hits
    		//pl = append(pl, plink)
    		//pi = append(pi, ptime, string(phits))
    		files = []File{
    			File{
    			Created: file.Created,
    			Filename: flink,
    			Hits: fhits,
    			},
    		}
	        return nil
	    })
	    return nil
	})

	/*
	for _, p := range pfiles {
		plink := string(p.Name())
		ptime := p.ModTime().String()
		psize := strconv.FormatInt(p.Size(), 8)
		pl = append(pl, plink)
		pi = append(pi, ptime, psize)
	}
	*/
	paste := &Paste{}
	var pastes []Paste
	//Lets try this with boltDB now!
	Db.View(func(tx *bolt.Tx) error {
	    b := tx.Bucket([]byte("Pastes"))
	    b.ForEach(func(k, v []byte) error {
	        //fmt.Printf("key=%s, value=%s\n", k, v)
	        err := json.Unmarshal(v, &paste)
    		if err != nil {
    			log.Println(err)
    		}
    		plink := paste.Title
    		//ptime, _ := time.Parse(timestamp, paste.Created.String())
    		phits := paste.Hits
    		//pl = append(pl, plink)
    		//pi = append(pi, ptime, string(phits))
    		pastes = []Paste{
    			Paste{
    			Created: paste.Created,
    			Title: plink,
    			Hits: phits,
    			},
    		}
	        return nil
	    })
	    return nil
	})


	short := &Shorturl{}
	var shorts []Shorturl
	//Lets try this with boltDB now!
	Db.View(func(tx *bolt.Tx) error {
	    b := tx.Bucket([]byte("Shorturls"))
	    b.ForEach(func(k, v []byte) error {
	        //fmt.Printf("key=%s, value=%s\n", k, v)
	        err := json.Unmarshal(v, &short)
    		if err != nil {
    			log.Println(err)
    		}
    		shortU := short.Short
    		//ptime, _ := time.Parse(timestamp, paste.Created.String())
    		longU := short.Long
    		hits := short.Hits
    		//pl = append(pl, plink)
    		//pi = append(pi, ptime, string(phits))
    		shorts = []Shorturl{
    			Shorturl{
    			Created: short.Created,
    			Short: shortU,
    			Long: longU,
    			Hits: hits,
    			},
    		}
	        return nil
	    })
	    return nil
	})

	return &ListPage{Page: page, Snips: snips, Pastes: pastes, Files: files, Shorturls: shorts}, nil
}


func listHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "listHandler")
	//vars := mux.Vars(r)
	//page := vars["page"]
	username := getUsername(w, r)
	l, _ := loadListPage(username)
	//fmt.Fprintln(w, l)

	err := renderTemplate(w, "list.tmpl", l)
	if err != nil {
		log.Println(err)
	}
	//log.Println("List rendered!")
	//timer.Step("list page rendered")
	//log.Println(l)
}

func remoteDownloadHandler(w http.ResponseWriter, r *http.Request) {
	remoteURL := r.FormValue("remote")
	fileURL, err := url.Parse(remoteURL)
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
	dlpath := "./data/uploads/"
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
	resp, err := check.Get(remoteURL)
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
        Created: time.Now().Format(timestamp),
        Filename: fileName,
    }
    err = fi.save()
    if err != nil {
        log.Println(err)
    }

	//fmt.Printf("%s with %v bytes downloaded", fileName, size)
	fmt.Fprintf(w, "%s with %v bytes downloaded", fileName, size)
	log.Println("Filename:")
	log.Println(fileName)
}

func putHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	contentLength := r.ContentLength
	var reader io.Reader
	var f io.WriteCloser
	var err error
	reader = r.Body
	if contentLength == -1 {
		// queue file to disk, because s3 needs content length
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

	contentType := r.Header.Get("Content-Type")

	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(vars["filename"]))
	}

	filename := sanitize.Path(filepath.Base(vars["filename"]))
	if filename == "." {
		//filename := sanitize.Path(filepath.Base(vars["filename"]))
		log.Println("Filename is blank " + filename)
		dictionary := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
		var bytes = make([]byte, 4)
		rand.Read(bytes)
		for k, v := range bytes {
			bytes[k] = dictionary[v%byte(len(dictionary))]
		}
		filename = string(bytes)
	}
	log.Printf("Uploading %s %d %s", filename, contentLength, contentType)
	path := "./data/uploads/"
	if f, err = os.OpenFile(filepath.Join(path, filename), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600); err != nil {
		fmt.Printf("%s", err.Error())
		http.Error(w, errors.New("Could not save file").Error(), 500)
		return
	}
	defer f.Close()
	if _, err = io.Copy(f, reader); err != nil {
		return
	}
	// w.Statuscode = 200

	//BoltDB stuff
    fi := &File{
        Created: time.Now().Format(timestamp),
        Filename: filename,
    }
    err = fi.save()
    if err != nil {
        log.Println(err)
    }

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, r.Header.Get("Scheme")+"://"+r.Host+"/d/%s\n", filename)
}

func (f *File) save() error {
    Db.Update(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte("Files"))
        encoded, err := json.Marshal(f)
        if err != nil {
            return err
        }
        return b.Put([]byte(f.Filename), encoded)
    })
    log.Println("FILE SAVED")
    return nil
}

//Short URL Handlers
func shortUrlHandler(w http.ResponseWriter, r *http.Request) {

	defer timeTrack(time.Now(), "shortUrlHandler")
	vars := mux.Vars(r)
	//username := getUsername(w, r)
	title := vars["short"]
	shorturl := &Shorturl{}
	//p, err := loadPage(title, username)
	//err = Db.View(func(tx *bolt.Tx) error {
	err := Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Shorturls"))
    	v := b.Get([]byte(title))
    	//Because BoldDB's View() doesn't return an error if there's no key found, just throw a 404 on nil
    	//After JSON Unmarshal, Content should be in paste.Content field
    	if v == nil {
			//http.Redirect(w, r, "/+edit/"+title, http.StatusFound)
			//http.NotFound(w, r)
			//http.Redirect(w, r, "https://m.jba.io", 302)
			http.Error(w, "Error 400 - No such domain at this address", 400)
			err := errors.New("No Such Short URL")
			return err
			//return err
			//log.Println(err)
    	} else {
    		err := json.Unmarshal(v, &shorturl)
    		if err != nil {
    			log.Println(err)
    		}
	        count := (shorturl.Hits + 1)
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

func shortUrlFormHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "shortUrlFormHandler")
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
	    Created: time.Now().Format(timestamp),
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
    fmt.Fprintln(w, "Your Short URL is available at: %s", s.Short)
	log.Println("Short: " + s.Short)
	log.Println("Long: " + s.Long)
}

func (s *Shorturl) save() error {
	Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Shorturls"))
	    encoded, err := json.Marshal(s)
	    if err != nil {
	    	return err
	    }
	    return b.Put([]byte(s.Short), encoded)
	})
	log.Println("SHORTURL SAVED")
	return nil
}

//Pastebin handlers
func pasteUpHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "pasteUpHandler")
	vars := mux.Vars(r)
	log.Println("Paste request...")
	log.Println(r.Header.Get("Scheme"))
	paste := r.Body
	buf := new(bytes.Buffer)
	buf.ReadFrom(paste)
	bpaste := buf.String()
	var name = ""
	if vars["id"] != "" {
		name = vars["id"]
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
	    Created: time.Now().Format(timestamp),
	    Title: name,
	    Content: bpaste,
	}
	err := p.save()
	if err != nil {
		log.Println(err)
	}
	fmt.Fprintln(w, r.Header.Get("Scheme")+"://"+r.Host+"/p/"+name)
	//log.Println(r.Header.Get("Scheme"))
	log.Println("Paste written to ./data/paste/" + name)
}

func pasteFormHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "pasteFormHandler")
	//vars := mux.Vars(r)
	//var name = ""
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
	    Created: time.Now().Format(timestamp),
	    Title: title,
	    Content: paste,
	}
	err = p.save()
	if err != nil {
		log.Println(err)
	}
	http.Redirect(w, r, r.Header.Get("Scheme")+"://"+r.Host+"/p/"+title, 302)
	log.Println("Paste written to ./data/paste/" + title)
	log.Println(r.Header.Get("Scheme")+"://"+r.Host+"/p/"+title)
}

func (p *Paste) save() error {
	Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Pastes"))
	    encoded, err := json.Marshal(p)
	    if err != nil {
	    	return err
	    }
	    return b.Put([]byte(p.Title), encoded)
	})
	log.Println("+PASTE SAVED")
	return nil
}

func pasteHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "pasteHandler")
	vars := mux.Vars(r)
	title := vars["id"]
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
    		//Still using Bluemonday for XSS protection, so some HTML elements can be rendered
    		//Can use template.HTMLEscapeString() if I wanted, which would simply escape stuff
	   		safe := bluemonday.UGCPolicy().Sanitize(paste.Content)
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
func editSnipHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "editSnipHandler")
	//title, err := getTitle(w, r)
	vars := mux.Vars(r)
	title := vars["page"]
	username := getUsername(w, r)
	snip := &Snip{}
	p, err := loadPage(title, username)
	if err != nil {
		log.Println(err)
	}
	err = Db.View(func(tx *bolt.Tx) error {
    	v := tx.Bucket([]byte("Snips")).Get([]byte(title))
    	//Because BoldDB's View() doesn't return an error if there's no key found, just render an empty page to edit
    	//After JSON Unmarshal, Content should be in paste.Content field
    	if v == nil {
			p = &Page{Title: title, UN: username}
			s := &Snip{Created: time.Now().Format(timestamp), Title: title,}
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

func snipHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "snipHandler")
	vars := mux.Vars(r)
	username := getUsername(w, r)
	title := vars["page"]
	snip := &Snip{}
	p, err := loadPage(title, username)
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

func saveSnipHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "saveSnipHandler")
	//title, err := getTitle(w, r)
	vars := mux.Vars(r)
	title := vars["page"]
	body := r.FormValue("body")
	//fmattertitle := r.FormValue("fmatter-title")
	fmattercats := r.FormValue("fmatter-cats")
	//fmatter := r.FormValue("fmatter")
	//newbody := strings.Replace(body, "\r", "", -1)
	bodslice := []string{}
	bodslice = append(bodslice, body)
	s := &Snip{
	    Created: time.Now().Format(timestamp),
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
	//timer.Step("wiki page saved")
}

func appendSnipHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "appendSnipHandler")
	//title, err := getTitle(w, r)
	vars := mux.Vars(r)
	title := vars["page"]
	body := r.FormValue("append")
	//newbody := strings.Replace(body, "\r", "", -1)

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
		log.Println("SNIP APPENDED")
		log.Println(encoded)
		log.Println(s)
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
	    	return err
	    }
		log.Println("SNIP SAVED")
		log.Println(encoded)
		log.Println(s)
	    return b.Put([]byte(s.Title), encoded)
	})
	if err != nil {
		log.Println(err)
	}
	return nil
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "downloadHandler")
	vars := mux.Vars(r)
    name := vars["name"]
    fpath := "./data/uploads/" + path.Base(name)

    //Attempt to increment file hit counter...
    file := &File{}
    Db.Update(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte("Files"))
        v := b.Get([]byte(name))
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
        return b.Put([]byte(name), encoded)
    })
    http.ServeFile(w, r, fpath)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	//Requests should come in on /api/delete/{type}/{name}
	vars := mux.Vars(r)
	ftype := vars["type"]
	fname := vars["name"]
	if ftype == "snip" {
		err := Db.Update(func(tx *bolt.Tx) error {
			log.Println(ftype + fname + " has been deleted")
		    return tx.Bucket([]byte("Snips")).Delete([]byte(fname))
		})
		if err != nil {
			log.Println(err)
			return
		}

	} else if ftype == "file" {
		err := Db.Update(func(tx *bolt.Tx) error {
			log.Println(ftype + fname + " has been deleted")
		    return tx.Bucket([]byte("Files")).Delete([]byte(fname))
		})
		if err != nil {
			log.Println(err)
			return
		}
		fpath := "./data/uploads/" + fname
		log.Println(fpath + " has been deleted")
		err = os.Remove(fpath)
		if err != nil {
			log.Println(err)
			return
		}
		fmt.Fprintf(w, fpath + " has been deleted")
	} else if ftype == "paste" {
		err := Db.Update(func(tx *bolt.Tx) error {
			log.Println(ftype + fname + " has been deleted")
		    return tx.Bucket([]byte("Pastes")).Delete([]byte(fname))
		})
		if err != nil {
			log.Println(err)
		}
		log.Println(fname + " has been deleted")
	} else if ftype == "shorturl" {
		err := Db.Update(func(tx *bolt.Tx) error {
			log.Println(ftype + fname + " has been deleted")
		    return tx.Bucket([]byte("Shorturls")).Delete([]byte(fname))
		})
		if err != nil {
			log.Println(err)
		}
		log.Println(fname + " has been deleted")
	} else {
		fmt.Fprintf(w, "Whatcha trying to do...")
	}
}

//func notFoundHandler(w http.ResponseWriter, r *http.Request) {}

func handleAdmin(w http.ResponseWriter, r *http.Request) {
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
    	//username := getUsername(w, r)
		//fmt.Fprintln(w, l)
		err = renderTemplate(w, "admin.tmpl", d)
		if err != nil {
			log.Println(err)
		}
		//log.Println("Admin page rendered!")
	}
}

func ping(w http.ResponseWriter, r *http.Request) {
	//url := "google.com"
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
	}
	url := r.PostFormValue("ping")
	username := getUsername(w, r)
	out, err := exec.Command("ping", "-c5", url).Output()
	if err != nil {
		log.Println(err)
	}
	outs := string(out)
	//fmt.Fprintln(w, "%s", outs)
	title := "TKOT - Pinging " + url
	p, err := loadPage(title, username)
	data := struct {
		Page *Page
	    Title string
	    UN string
	    Message string
	} {
		p,
		title,
		username,
		outs,
	}
	err = renderTemplate(w, "lg.tmpl", data)
	if err != nil {
		log.Println(err)
	}
}


func newSnipFormHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "newSnipFormHandler")
	//vars := mux.Vars(r)
	//var name = ""
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
	}
	title := r.PostFormValue("newsnip")
	//http.Redirect(w, r, r.Header.Get("Scheme")+"://"+r.Host+"/+edit/"+title, 302)
	http.Redirect(w, r, "/+edit/"+title, http.StatusFound)
	//log.Println("Paste written to ./data/paste/" + title)
	//log.Println(r.Header.Get("Scheme")+"://"+r.Host+"/p/"+title)
	log.Println("New Snip at "+title+" created from search box")
}

type Logger struct {
	// Logger inherits from log.Logger used to log messages with the Logger middleware
	*log.Logger
}

// NewLogger returns a new Logger instance
func NewMyLogger() *Logger {
	return &Logger{log.New(os.Stdout, "[negroni] ", 0)}
}

func (l *Logger) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	start := time.Now()
	l.Printf("Started %s %s | Host: %s | Raw URL: %s | UserAgent: %s | HTTPS: %s | IP: %s", r.Method, r.URL.Path, r.Host, r.Header.Get("X-Raw-URL"), r.Header.Get("User-Agent"), r.Header.Get("Scheme"), r.Header.Get("X-Forwarded-For"))

	next(rw, r)

	res := rw.(negroni.ResponseWriter)
	l.Printf("Completed %v %s in %v", res.Status(), http.StatusText(res.Status()), time.Since(start))
}

func main() {
	/* for reference
	p1 := &Page{Title: "TestPage", Body: []byte("This is a sample page.")}
	p1.save()
	p2, _ := loadPage("TestPage")
	fmt.Println(string(p2.Body))
	*/

	//var db, _ = bolt.Open("./data/bolt.db", 0600, nil)
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
		return nil
	})

	Db.View(func(tx *bolt.Tx) error {
    	b := tx.Bucket([]byte("Pastes"))
    	b.ForEach(func(k, v []byte) error {
        	fmt.Printf("key=%s, value=%s\n", k, v)
        	return nil
    	})
    	c := tx.Bucket([]byte("Files"))
    	c.ForEach(func(k, v []byte) error {
        	fmt.Printf("key=%s, value=%s\n", k, v)
        	return nil
    	})
    	d := tx.Bucket([]byte("Snips"))
    	d.ForEach(func(k, v []byte) error {
        	fmt.Printf("key=%s, value=%s\n", k, v)
        	return nil
    	})
    	e := tx.Bucket([]byte("Shorturls"))
    	e.ForEach(func(k, v []byte) error {
        	fmt.Printf("key=%s, value=%s\n", k, v)
        	return nil
    	})
    	return nil
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	//var err error
	//httpauth
	os.Create(backendfile)
	//defer os.Remove(backendfile)
	backend, err := httpauth.NewGobFileAuthBackend(backendfile)
	if err != nil {
		panic(err)
	}

	roles = make(map[string]httpauth.Role)
	roles["user"] = 1
	roles["gator"] = 2
	roles["admin"] = 10

	dictionary := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	rb := make([]byte, 32)
	rand.Read(rb)
	for k, v := range rb {
		rb[k] = dictionary[v%byte(len(dictionary))]
	}
	sess_id := string(rb)
	log.Println("Session ID: " + sess_id)

	aaa, err = httpauth.NewAuthorizer(backend, []byte("ieP2Aengoovu4AhZeimoo"), "user", roles)
	if err != nil {
		panic(err)
	}
	//THIS SHOULD BE IN FORM OF: []byte("userpass")
	//hash, err := bcrypt.GenerateFromPassword([]byte("***REMOVED******REMOVED***"), 8)
	hash, err := bcrypt.GenerateFromPassword([]byte(myUn + myPw), 8)
	if err != nil {
		panic(err)
	}
	defaultUser := httpauth.UserData{Username: myUn, Email: myEmail, Hash: hash, Role:"admin"}
	err = backend.SaveUser(defaultUser)
	if err != nil {
		panic(err)
	}

	/*
	users, err := backend.Users()
	if err != nil {
		panic(err)
	}
	log.Println("USERS:")
	log.Println(users)
	*/

	flag.Parse()

	r := mux.NewRouter()
	gen := r.Host("go.jba.io").Subrouter()
	if fLocal {
		gen = r.Host("go.dev").Subrouter()
		log.Println("Listening on .dev domains due to -l flag...")
	}
	//w := r.PathPrefix("/+").Subrouter()
	gen.HandleFunc(`/+edit/{page}`, GuardPath(editSnipHandler)).Methods("GET")
	gen.HandleFunc(`/+raw/{page}`, rawSnipHandler).Methods("GET")

	//Short URL router
	//short := r.Host("s.es.gy").Subrouter()
	s := r.Host("{short}.es.gy").Subrouter()	

	if fLocal {
		//short = r.Host("short.dev").Subrouter()
		s = r.Host("{short}.dev").Subrouter()
	}
	//short.HandleFunc("/", shortenPageHandler)
	gen.HandleFunc("/s", shortenPageHandler)
	//Short URL wildcard subdomain router

	// Only matches if domain is "www.domain.com".
	//s.Host("s.es.gy").HandleFunc("/{short}", shortUrlHandler).Methods("GET")
	// Matches a dynamic subdomain.
	s.HandleFunc("/", shortUrlHandler).Methods("GET", "HEAD")



	//API Functions
	api := gen.PathPrefix("/api").Subrouter()
	api.HandleFunc(`/delete/{type}/{name}`, GuardPath(deleteHandler)).Methods("GET")
	//Wiki API calls
	api.HandleFunc("/wiki/new", GuardPath(newSnipFormHandler)).Methods("POST", "PUT")
	api.HandleFunc(`/wiki/new/{page:[0-9a-zA-Z\_\-]+($|\/[0-9a-zA-Z\_\-]+)}`, GuardPath(saveSnipHandler)).Methods("POST", "PUT")
	api.HandleFunc(`/wiki/append/{page:[0-9a-zA-Z\_\-]+($|\/[0-9a-zA-Z\_\-]+)}`, GuardPath(appendSnipHandler)).Methods("POST")
	//Paste API calls
	api.HandleFunc("/paste/new", pasteFormHandler).Methods("POST")
	//File API calls
	api.HandleFunc("/file/new", putHandler).Methods("POST")
	api.HandleFunc("/file/remote", remoteDownloadHandler).Methods("POST")
	//User API calls
	api.HandleFunc("/user/new", GuardPath(addUser)).Methods("POST")
	//Short URL calls
	api.HandleFunc("/shorten/new", shortUrlFormHandler).Methods("POST")
	//Looking glass calls
	api.HandleFunc("/ping", ping)

	//Looking Glass
	gen.HandleFunc("/lg", lgHandler).Methods("GET")

	//Auth
	gen.HandleFunc("/login", loginHandler).Methods("POST")
	gen.HandleFunc("/login", loginPageHandler).Methods("GET")
	gen.HandleFunc("/logout", logoutHandler).Methods("POST", "GET")

	//Pastebin functions
	gen.HandleFunc("/p", pastePageHandler).Methods("GET")
	gen.HandleFunc("/p/{id}", pasteHandler).Methods("GET")

	//Pastebin API, kept on the same route for accessibility from CLI
	gen.HandleFunc("/p/{id}", pasteUpHandler).Methods("PUT", "POST")
	gen.HandleFunc("/p", pasteUpHandler).Methods("POST")
	gen.HandleFunc("/p/", pasteUpHandler).Methods("POST")

	//Upload functions
	gen.HandleFunc("/up", putHandler).Methods("POST", "PUT")
	gen.HandleFunc("/up/{id}", putHandler).Methods("PUT", "POST")
	gen.HandleFunc("/up", uploadPageHandler).Methods("GET")
	gen.HandleFunc("/up/", uploadPageHandler).Methods("GET")

	//r.HandleFunc("/priv", privHandler)
	gen.HandleFunc("/admin", GuardAdminPath(handleAdmin))
	gen.HandleFunc("/search/{term}", searchHandler)
	gen.HandleFunc("/short", shortenPageHandler)

	//List pages and stuff
	//r.HandleFunc("/list/{page}", listHandler).Methods("GET")
	gen.HandleFunc("/list", listHandler).Methods("GET")

	//Download files
	gen.HandleFunc("/d/{name}", downloadHandler).Methods("GET")

	gen.HandleFunc("/s/{short}", shortUrlHandler).Methods("GET", "HEAD")

	//Wiki functions
	gen.HandleFunc(`/{page}`, snipHandler).Methods("GET")
	//r.HandleFunc(`/{page}/`, snipHandler).Methods("GET")

	//Index
	gen.HandleFunc("/", indexHandler).Methods("GET")

	//n := negroni.Classic()
	n := negroni.New(negroni.NewRecovery(), NewMyLogger(), negroni.NewStatic(http.Dir("public")))
	n.UseHandler(r)
	//n.UseHandler(s)
	n.Run(":" + port)


}
