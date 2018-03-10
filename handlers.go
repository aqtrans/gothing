package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"

	raven "github.com/getsentry/raven-go"
	"github.com/gorilla/mux"
	"github.com/microcosm-cc/bluemonday"

	"github.com/boltdb/bolt"
	//"github.com/gorilla/mux"
	//"github.com/dimfeld/httptreemux"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/kennygrant/sanitize"
	"github.com/spf13/viper"
	"jba.io/go/httputils"
	//"jba.io/go/auth"
)

func (env *thingEnv) indexHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "indexHandler")
	title := "index"
	p, _ := loadMainPage(title, w, r)
	err := renderTemplate(env, w, "index.tmpl", p)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
}

func (env *thingEnv) helpHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "helpHandler")
	title := "Help"
	p, _ := loadMainPage(title, w, r)
	err := renderTemplate(env, w, "help.tmpl", p)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
}

func (env *thingEnv) loadGalleryPage(w http.ResponseWriter, r *http.Request) (*GalleryPage, error) {
	defer httputils.TimeTrack(time.Now(), "loadGalleryPage")
	page, perr := loadPage("Gallery", w, r)
	if perr != nil {
		raven.CaptureError(perr, nil)
		log.Println(perr)
	}

	db := env.getDB()
	defer env.closeDB()

	var images []*Image
	//Lets try this with boltDB now!
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Images"))
		b.ForEach(func(k, v []byte) error {
			//fmt.Printf("key=%s, value=%s\n", k, v)
			var image *Image
			err := json.Unmarshal(v, &image)
			if err != nil {
				raven.CaptureError(err, nil)
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

func (env *thingEnv) galleryHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "galleryHandler")
	l, err := env.loadGalleryPage(w, r)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}

	err = renderTemplate(env, w, "gallery.tmpl", l)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
}

func (env *thingEnv) galleryEsgyHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "galleryEsgyHandler")
	l, err := env.loadGalleryPage(w, r)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}

	err = renderTemplate(env, w, "gallery-esgy.tmpl", l)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
}

func (env *thingEnv) adminListHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "galleryListHandler")
	//title := "Admin List"
	l, err := env.loadGalleryPage(w, r)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
	err = renderTemplate(env, w, "admin_list.tmpl", l)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
}

func (env *thingEnv) adminHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "adminHandler")
	title := "Admin Panel"
	p, _ := loadMainPage(title, w, r)
	err := renderTemplate(env, w, "admin.tmpl", p)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
}

func (env *thingEnv) adminSignupHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "adminSignupHandler")
	title := "Admin Signup"
	p, _ := loadMainPage(title, w, r)
	err := renderTemplate(env, w, "admin_user.tmpl", p)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
}

func (env *thingEnv) adminUserPassHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "adminUserPassHandler")
	title := "Admin Password Change"
	p, _ := loadMainPage(title, w, r)
	err := renderTemplate(env, w, "admin_password.tmpl", p)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
}

func (env *thingEnv) signupPageHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "adminSignupHandler")
	title := "Signup"
	p, _ := loadMainPage(title, w, r)
	err := renderTemplate(env, w, "signup.tmpl", p)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
}

func (env *thingEnv) lgHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "lgHandler")
	title := "lg"
	p, err := loadPage(title, w, r)
	data := struct {
		Page    *Page
		Title   string
		Message string
	}{
		p,
		title,
		"",
	}
	err = renderTemplate(env, w, "lg.tmpl", data)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
}

func (env *thingEnv) searchHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "searchHandler")
	params := mux.Vars(r)
	term := params["name"]
	sterm := regexp.MustCompile(term)

	file := &File{}
	paste := &Paste{}

	db := env.getDB()
	defer env.closeDB()

	//Lets try this with boltDB now!
	db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte("Pastes"))
		c.ForEach(func(k, v []byte) error {
			//fmt.Printf("key=%s, value=%s\n", k, v)
			err := json.Unmarshal(v, &paste)
			if err != nil {
				raven.CaptureError(err, nil)
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
				raven.CaptureError(err, nil)
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

func (env *thingEnv) uploadPageHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "uploadPageHandler")
	title := "up"
	p, _ := loadMainPage(title, w, r)
	err := renderTemplate(env, w, "up.tmpl", p)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
}

func (env *thingEnv) uploadImagePageHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "uploadImagePageHandler")
	title := "upimg"
	p, _ := loadMainPage(title, w, r)
	err := renderTemplate(env, w, "upimg.tmpl", p)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
}

func (env *thingEnv) pastePageHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "pastePageHandler")
	title := "paste"
	p, _ := loadMainPage(title, w, r)
	err := renderTemplate(env, w, "paste.tmpl", p)
	r.ParseForm()
	//log.Println(r.Form)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
}

func (env *thingEnv) shortenPageHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "shortenPageHandler")
	title := "shorten"
	p, _ := loadMainPage(title, w, r)
	err := renderTemplate(env, w, "shorten.tmpl", p)
	r.ParseForm()
	//log.Println(r.Form)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
}

func (env *thingEnv) loginPageHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "loginPageHandler")
	title := "login"
	p, err := loadPage(title, w, r)
	data := struct {
		Page  *Page
		Title string
	}{
		p,
		title,
	}
	err = renderTemplate(env, w, "login.tmpl", data)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		return
	}
}

func (env *thingEnv) listHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "listHandler")
	l, err := env.loadListPage(w, r)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
	err = renderTemplate(env, w, "list.tmpl", l)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
}

//Short URL Handler
func (env *thingEnv) shortUrlHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "shortUrlHandler")
	shorturl := &Shorturl{}
	params := mux.Vars(r)
	title := strings.ToLower(params["name"])

	db := env.getDB()
	defer env.closeDB()

	if title == "www" {
		//indexHandler(w, r)
		http.Redirect(w, r, "//"+viper.GetString("MainTLD"), http.StatusTemporaryRedirect)
		return
	}
	/*
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
		}*/

	var destURL string

	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Shorturls"))
		v := b.Get([]byte(title))
		//Because BoldDB's View() doesn't return an error if there's no key found, just throw a 404 on nil
		//After JSON Unmarshal, Content should be in paste.Content field
		if v == nil {
			//http.Error(w, "Error 400 - No such domain at this address", http.StatusBadRequest)
			err := errors.New(title + " - No Such Short URL")
			return err
			//log.Println(err)
		}
		err := json.Unmarshal(v, &shorturl)
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println(err)
			return err
		}

		destURL = shorturl.Long

		// If shorturl.Long begins with /, assume it is a file/image/screenshot to be served locally
		//    This is to replace the rest of the if/else now-commented out:
		if strings.HasPrefix(shorturl.Long, "/") {
			destURL = "//" + viper.GetString("MainTLD") + shorturl.Long
			//http.Redirect(w, r, "//"+viper.GetString("MainTLD")+shorturl.Long, 302)
		}

		/*
			//If the shorturl is local, just serve whatever file being requested
			if strings.Contains(shorturl.Long, viper.GetString("ShortTLD")+"/") {
				log.Println("LONG URL CONTAINS ShortTLD")
				if strings.HasPrefix(shorturl.Long, "http://"+viper.GetString("ImageTLD")) {
					u, err := url.Parse(shorturl.Long)
					if err != nil {
						log.Println(err)
					}
					segments := strings.Split(u.Path, "/")
					fileName := segments[len(segments)-1]
					log.Println("Serving " + shorturl.Long + " file directly")
					http.ServeFile(w, r, filepath.Join(viper.GetString("ImgDir"), fileName))
				}
				if strings.HasPrefix(shorturl.Long, "https://"+viper.GetString("ImageTLD")) {
					u, err := url.Parse(shorturl.Long)
					if err != nil {
						log.Println(err)
					}
					segments := strings.Split(u.Path, "/")
					fileName := segments[len(segments)-1]
					log.Println("Serving " + shorturl.Long + " file directly")
					http.ServeFile(w, r, filepath.Join(viper.GetString("ImgDir"), fileName))
				}
			} else if strings.Contains(shorturl.Long, viper.GetString("MainTLD")+"/i/") {
				log.Println("LONG URL CONTAINS MainTLD")
				if strings.HasPrefix(shorturl.Long, "http://"+viper.GetString("MainTLD")+"/i/") {
					u, err := url.Parse(shorturl.Long)
					if err != nil {
						log.Println(err)
					}
					segments := strings.Split(u.Path, "/")
					fileName := segments[len(segments)-1]
					log.Println("Serving " + shorturl.Long + " file directly")
					http.ServeFile(w, r, filepath.Join(viper.GetString("ImgDir"), fileName))
				}
			} else {
				destURL := shorturl.Long
				// If the destination is not a full URL, make it so
				if !strings.HasPrefix(destURL, "http") {
					destURL = "http://" + destURL
				}
				http.Redirect(w, r, destURL, 302)
			}
		*/

		s := &Shorturl{
			Created: shorturl.Created,
			Short:   shorturl.Short,
			Long:    shorturl.Long,
			Hits:    shorturl.Hits + 1,
		}
		encoded, err := json.Marshal(s)
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println(err)
			return err
		}

		return b.Put([]byte(title), encoded)
	})
	if err != nil {
		raven.CaptureError(err, map[string]string{"requestURI": r.RequestURI})
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		if !strings.HasPrefix(destURL, "http") {
			destURL = "http://" + destURL
		}
		http.Redirect(w, r, destURL, http.StatusSeeOther)
	}
}

func (env *thingEnv) pasteHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "pasteHandler")
	params := mux.Vars(r)
	title := params["name"]
	paste := &Paste{}
	db := env.getDB()
	defer env.closeDB()
	err := db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket([]byte("Pastes")).Get([]byte(title))
		//Because BoldDB's View() doesn't return an error if there's no key found, just throw a 404 on nil
		//After JSON Unmarshal, Content should be in paste.Content field
		if v == nil {
			return errors.New("Paste does not exist")
		}
		err := json.Unmarshal(v, &paste)
		if err != nil {
			return err
		}
		//No longer using BlueMonday or template.HTMLEscapeString because theyre too overzealous
		//I need '<' and '>' in tact for scripts and such

		//safe := template.HTMLEscapeString(paste.Content)
		//safe := sanitize.HTML(paste.Content)

		//safe := strings.Replace(paste.Content, "<script>", "< script >", -1)

		//safe := paste.Content

		// Bluemonday
		p := bluemonday.UGCPolicy()
		safe := p.Sanitize(paste.Content)

		fmt.Fprintf(w, "%s", safe)
		return nil
	})
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		http.NotFound(w, r)
		return
	}

	//Attempt to increment paste hit counter...
	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Pastes"))
		v := b.Get([]byte(title))
		//If there is no existing key, do not do a thing
		if v == nil {
			http.NotFound(w, r)
			return nil
		}
		err := json.Unmarshal(v, &paste)
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println(err)
		}
		count := (paste.Hits + 1)
		p := &Paste{
			Created: paste.Created,
			Title:   paste.Title,
			Content: paste.Content,
			Hits:    count,
		}
		encoded, err := json.Marshal(p)
		return b.Put([]byte(title), encoded)
	})
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
}

func (env *thingEnv) downloadHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "downloadHandler")
	params := mux.Vars(r)
	name := params["name"]
	fpath := filepath.Join(viper.GetString("FileDir"), path.Base(name))
	//fpath := cfg.FileDir + path.Base(name)

	db := env.getDB()
	defer env.closeDB()

	//Attempt to increment file hit counter...
	file := &File{}
	db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Files"))
		v := b.Get([]byte(name))
		//If there is no existing key, do not do a thing
		if v == nil {
			http.NotFound(w, r)
			return nil
		}
		err := json.Unmarshal(v, &file)
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println(err)
		}
		count := (file.Hits + 1)
		fi := &File{
			Created:  file.Created,
			Filename: file.Filename,
			Hits:     count,
		}
		encoded, err := json.Marshal(fi)
		http.ServeFile(w, r, fpath)
		return b.Put([]byte(name), encoded)
	})

}

func (env *thingEnv) downloadImageHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "downloadImageHandler")
	params := mux.Vars(r)
	name := params["name"]
	//fpath := cfg.ImgDir + path.Base(name)
	fpath := filepath.Join(viper.GetString("ImgDir"), path.Base(name))

	if name == "favicon.ico" {
		//log.Println("omg1")
		http.NotFound(w, r)
		return
	}
	if name == "favicon.png" {
		//log.Println("omg2")
		http.NotFound(w, r)
		return
	}

	extensions := []string{".mp4", ".webm", ".gif", ".jpg", ".jpeg", ".png"}
	//If this is extensionless, search for the proper file with the extension
	//  Note: Searching for mp4, webm first
	if filepath.Ext(name) == "" {
		//log.Println("NO EXTENSION FOUND OMG")
		for _, ext := range extensions {
			if _, err := os.Stat(fpath + ext); err == nil {
				name = name + ext
				//fpath = cfg.ImgDir + path.Base(name)
				fpath = filepath.Join(viper.GetString("ImgDir"), path.Base(name))
				break
			}
		}
	}
	db := env.getDB()
	defer env.closeDB()

	//Attempt to increment file hit counter...
	image := &Image{}
	db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Images"))
		v := b.Get([]byte(name))
		//If there is no existing key, do not do a thing
		if v == nil {
			//http.NotFound(w, r)
			//log.Println("omg3")
			return nil
		}
		err := json.Unmarshal(v, &image)
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println(err)
		}
		count := (image.Hits + 1)
		imi := &Image{
			Created:  image.Created,
			Filename: image.Filename,
			Hits:     count,
		}
		encoded, err := json.Marshal(imi)
		return b.Put([]byte(name), encoded)
	})

	// Try and intercept GIF requests if a fpath.webm
	if filepath.Ext(name) == ".gif" {
		nameWithoutExt := name[0 : len(name)-len(filepath.Ext(".gif"))]
		// Check for existence of nameWithoutExt.mp4
		if _, err := os.Stat(filepath.Join(viper.GetString("ImgDir"), nameWithoutExt+".mp4")); err == nil {
			name = nameWithoutExt + ".mp4"
		}
		// Check for existence of nameWithoutExt.webm
		if _, err := os.Stat(filepath.Join(viper.GetString("ImgDir"), nameWithoutExt+".webm")); err == nil {
			name = nameWithoutExt + ".webm"
		}
	}

	//If this is an mp4 or webm file, serve it so it acts like a GIF
	if filepath.Ext(name) == ".mp4" {
		//w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!doctype html><html><head><title>` + name + `</title></head>
					    <body><video src=/imagedirect/` + name + ` autoplay loop muted></video></body>
					    </html>`))
	} else if filepath.Ext(name) == ".webm" {
		//w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!doctype html><html><head><title>` + name + `</title></head>
					    <body><video src=/imagedirect/` + name + ` autoplay loop muted></video></body>
					    </html>`))
	} else {
		http.ServeFile(w, r, fpath)
	}
}

//Separate function so thumbnail displays on the Gallery page do not increase hit counter
//TODO: Probably come up with a better way to do this, IP based exclusion perhaps?
func imageThumbHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("imageThumbHandler r.Host:", r.Host, r.RemoteAddr, r.Header)
	defer httputils.TimeTrack(time.Now(), "imageThumbHandler")
	params := mux.Vars(r)
	name := params["name"]
	fpath := filepath.Join(viper.GetString("ImgDir"), path.Base(strings.TrimSuffix(name, ".png")))
	thumbPath := filepath.Join(viper.GetString("ThumbDir"), path.Base(name))

	//log.Println("name:"+ name)
	//log.Println("fpath:"+ fpath)
	//log.Println("thumbpath:"+thumbPath)

	//Check to see if the large image already exists
	//If so, serve it directly
	if _, err := os.Stat(thumbPath); err == nil {
		log.Println("Pre-existing thumbnail already found, serving it...")
		http.ServeFile(w, r, filepath.Join(viper.GetString("ThumbDir"), path.Base(name)))
	} else {
		log.Println("Thumbnail not found. Running thumbnail function...")
		makeThumb(fpath, thumbPath)

		//gifsicle --conserve-memory --colors 256 --resize 2000x_ ./up-imgs/groove_fox.gif -o ./tmp/BIG-groove_fox.gif
		//convert -define "jpeg:size=300x300 -thumbnail 300x300 ./up-imgs/

		/*
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
		*/
		//Trying with imaging library now

		http.ServeFile(w, r, filepath.Join(viper.GetString("ThumbDir"), path.Base(name)))
	}
}

func serveContent(w http.ResponseWriter, r *http.Request, dir, file string) {
	f, err := http.Dir(dir).Open(file)
	if err != nil {
		raven.CaptureError(err, nil)
		http.NotFound(w, r)
		return
	}
	content := io.ReadSeeker(f)
	http.ServeContent(w, r, file, time.Now(), content)
	return
}

func imageDirectHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "imageDirectHandler")
	params := mux.Vars(r)
	name := params["name"]
	log.Println("imageDirectHandler request info:", r.Header, r.Host, r.RemoteAddr)
	serveContent(w, r, viper.GetString("ImgDir"), name)

}

// imageBigHandler uses a weird CSS trick to make the images really big
func imageBigHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "imageBigHandler")
	params := mux.Vars(r)
	name := params["name"]

	// Try and intercept GIF requests since they should all be MP4s now
	if filepath.Ext(name) == ".gif" {
		nameWithoutExt := name[0 : len(name)-len(filepath.Ext(".gif"))]
		// Check for existence of nameWithoutExt.mp4
		if _, err := os.Stat(filepath.Join(viper.GetString("ImgDir"), nameWithoutExt+".mp4")); err == nil {
			name = nameWithoutExt + ".mp4"
		}
		// Check for existence of nameWithoutExt.webm
		if _, err := os.Stat(filepath.Join(viper.GetString("ImgDir"), nameWithoutExt+".webm")); err == nil {
			name = nameWithoutExt + ".webm"
		}
	}

	//Check if small image exists:
	_, err := os.Stat(filepath.Join(viper.GetString("ImgDir"), path.Base(name)))
	if err != nil && !os.IsNotExist(err) {
		raven.CaptureError(err, nil)
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	switch imgExt(name) {
	case "mp4":
		w.Write([]byte(`<!doctype html>
			<html>
			<head>
			<link rel="stylesheet" href="/assets/css/thing.css">
			<meta charset=utf-8>
			<title>` + name + `</title>
			</head>
			<body>
			<video autoplay muted loop class="embiggened" onclick="this.paused?this.play():this.pause();">
			<source src='/imagedirect/` + name + `' type="video/mp4">
			</video>
			</body>
			</html>`))
	case "webm":
		w.Write([]byte(`<!doctype html>
				<html>
				<head>
				<link rel="stylesheet" href="/assets/css/thing.css">
				<meta charset=utf-8>
				<title>` + name + `</title>
				</head>
				<body>
				<video autoplay muted loop class="embiggened" onclick="this.paused?this.play():this.pause();">
				<source src='/imagedirect/` + name + `' type="video/webm">
				</video>
				</body>
				</html>`))
	default:
		w.Write([]byte(`<!doctype html>
				<html>
				<head>
				<link rel="stylesheet" href="/assets/css/thing.css">
				<meta charset=utf-8>
				<title>` + name + `</title>
				</head>
				<body>
				<img src='/imagedirect/` + name + `' class="embiggened">
				</body>
				</html>`))
	}
}

func (env *thingEnv) viewMarkdownHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "viewMarkdownHandler")
	params := mux.Vars(r)
	name := params["name"]
	p, err := loadPage(name, w, r)
	if err != nil {
		raven.CaptureError(err, nil)
		http.NotFound(w, r)
		return
	}

	body, err := ioutil.ReadFile("./md/" + name + ".md")
	if err != nil {
		raven.CaptureError(err, nil)
		http.NotFound(w, r)
		log.Println(err)
		return
	}
	//unsafe := blackfriday.MarkdownCommon(body)
	md := markdownRender(body)
	mdhtml := template.HTML(md)
	//html := bluemonday.UGCPolicy().SanitizeBytes(unsafe)

	data := struct {
		Page  *Page
		Title string
		MD    template.HTML
	}{
		p,
		name,
		mdhtml,
	}
	err = renderTemplate(env, w, "md.tmpl", data)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
	log.Println(name + " Page rendered!")
}

func (env *thingEnv) APInewRemoteFile(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "APInewRemoteFile")

	remoteURL := r.FormValue("remote")
	finURL := remoteURL
	if !strings.HasPrefix(remoteURL, "http") {
		log.Println("remoteURL does not contain a URL prefix, so adding http")
		finURL = "http://" + remoteURL
	}
	fileURL, err := url.Parse(finURL)
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
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
	dlpath := viper.GetString("FileDir")
	if r.FormValue("remote-file-name") != "" {
		fileName = sanitize.Name(r.FormValue("remote-file-name"))
		log.Println("custom remote file name: " + fileName)
	}
	file, err := os.Create(filepath.Join(dlpath, fileName))
	if err != nil {
		raven.CaptureError(err, nil)
		fmt.Println(err)
		env.authState.SetFlash("Failed to save remote file.", w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		panic(err)
	}
	defer file.Close()
	check := http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}
	resp, err := check.Get(finURL)
	if err != nil {
		raven.CaptureError(err, nil)
		fmt.Println(err)
		env.authState.SetFlash("Failed to save remote file.", w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		panic(err)
	}
	defer resp.Body.Close()
	fmt.Println(resp.Status)

	size, err := io.Copy(file, resp.Body)
	if err != nil {
		raven.CaptureError(err, nil)
		env.authState.SetFlash("Failed to save remote file.", w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		panic(err)
	}

	//BoltDB stuff
	fi := &File{
		Created:   time.Now().Unix(),
		Filename:  fileName,
		RemoteURL: finURL,
	}
	err = fi.save(env)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		env.authState.SetFlash("Failed to save remote file.", w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}

	//fmt.Printf("%s with %v bytes downloaded", fileName, size)
	//fmt.Fprintf(w, "%s with %v bytes downloaded from %s", fileName, size, finURL)
	fmt.Printf("%s with %v bytes downloaded from %s", fileName, size, finURL)

	env.authState.SetFlash("Successfully saved "+fileName+": https://"+viper.GetString("MainTLD")+"/d/"+fileName, w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (env *thingEnv) APInewFile(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "APInewFile")
	params := mux.Vars(r)
	name := params["name"]
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
	path := viper.GetString("FileDir")
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
		if !strings.HasPrefix(remoteURL, "http") {
			log.Println("remoteURL does not contain a URL prefix, so adding http")
			finURL = "http://" + remoteURL
		}
		fileURL, err := url.Parse(finURL)
		if err != nil {
			raven.CaptureErrorAndWait(err, nil)
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
			filename = sanitize.Name(r.FormValue("remote-file-name"))
			log.Println("custom remote file name: " + filename)
		}
		file, err := os.Create(filepath.Join(path, filename))
		if err != nil {
			raven.CaptureError(err, nil)
			env.authState.SetFlash("Failed to save file.", w)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			fmt.Println(err)
			panic(err)
		}
		defer file.Close()
		check := http.Client{
			CheckRedirect: func(r *http.Request, via []*http.Request) error {
				r.URL.Opaque = r.URL.Path
				return nil
			},
		}
		resp, err := check.Get(finURL)
		if err != nil {
			raven.CaptureError(err, nil)
			env.authState.SetFlash("Failed to save file.", w)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			fmt.Println(err)
			panic(err)
		}
		defer resp.Body.Close()
		fmt.Println(resp.Status)

		size, err := io.Copy(file, resp.Body)
		if err != nil {
			raven.CaptureError(err, nil)
			env.authState.SetFlash("Failed to save file.", w)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			panic(err)
		}

		//BoltDB stuff
		fi = &File{
			Created:   time.Now().Unix(),
			Filename:  filename,
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
				raven.CaptureError(err, nil)
				log.Printf("%s", err.Error())
				http.Error(w, err.Error(), 500)
				return
			}
			if n > _24K {
				file, err := ioutil.TempFile("./tmp/", "transfer-")
				if err != nil {
					raven.CaptureError(err, nil)
					log.Printf("%s", err.Error())
					http.Error(w, err.Error(), 500)
					return
				}
				defer file.Close()
				n, err = io.Copy(file, io.MultiReader(&b, f))
				if err != nil {
					raven.CaptureError(err, nil)
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
		filename = sanitize.Path(filepath.Base(name))
		//log.Println(filename)
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
			filename = sanitize.Name(r.FormValue("local-file-name"))
			log.Println("custom local file name: " + filename)
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
		contentType = mime.TypeByExtension(filepath.Ext(name))
		//BoltDB stuff
		fi = &File{
			Created:  time.Now().Unix(),
			Filename: filename,
		}
	} else if uptype == "form" {
		//log.Println("Content-type is "+contentType)
		err := r.ParseMultipartForm(_24K)
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println("ParseMultiform reader error")
			log.Println(err)
			env.authState.SetFlash("Failed to save file.", w)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		file, handler, err := r.FormFile("file")
		filename = handler.Filename
		defer file.Close()
		if err != nil {
			raven.CaptureError(err, nil)
			fmt.Println(err)
			env.authState.SetFlash("Failed to save file.", w)
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
		if r.FormValue("local-file-name") != "" {
			filename = sanitize.Name(r.FormValue("local-file-name"))
			log.Println("custom local file name: " + filename)
		}

		f, err := os.OpenFile(filepath.Join(path, filename), os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			raven.CaptureError(err, nil)
			fmt.Println(err)
			env.authState.SetFlash("Failed to save file.", w)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		defer f.Close()
		io.Copy(f, file)

		//BoltDB stuff
		fi = &File{
			Created:  time.Now().Unix(),
			Filename: filename,
		}
	}

	err = fi.save(env)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		env.authState.SetFlash("Failed to save file.", w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}

	if uptype == "cli" {
		fmt.Fprintf(w, "https://"+viper.GetString("MainTLD")+"/d/"+filename)
	} else {
		env.authState.SetFlash("Successfully saved "+filename+": https://"+viper.GetString("MainTLD")+"/d/"+filename, w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (env *thingEnv) APInewShortUrlForm(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "APInewShortUrlForm")
	err := r.ParseForm()
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		env.authState.SetFlash("Failed to shorten URL.", w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}

	subdomain := r.PostFormValue("shortSub")

	short := r.PostFormValue("short")
	long := r.PostFormValue("long")

	if subdomain == "" {
		if short == "" {
			dictionary := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
			var bytes = make([]byte, 4)
			rand.Read(bytes)
			for k, v := range bytes {
				bytes[k] = dictionary[v%byte(len(dictionary))]
			}
			short = string(bytes)
		}
		full := "https://" + viper.GetString("ShortTLD") + "/" + short
		log.Println("Subdomain is blank, creating a regular short URL.")
		log.Println(full)
		s := &Shorturl{
			Created: time.Now().Unix(),
			Short:   short,
			Long:    long,
		}

		err = s.save(env)
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println(err)
			env.authState.SetFlash("Failed to shorten URL.", w)
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
		//log.Println("Short: " + s.Short)
		//log.Println("Long: " + s.Long)

		env.authState.SetFlash("Successfully shortened "+s.Long, w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	log.Println("Subdomain is not blank, creating a subdomain short URL.")
	s := &Shorturl{
		Created: time.Now().Unix(),
		Short:   subdomain,
		Long:    long,
	}

	err = s.save(env)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		env.authState.SetFlash("Failed to shorten URL.", w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
	//log.Println("Short: " + s.Short)
	//log.Println("Long: " + s.Long)

	env.authState.SetFlash("Successfully shortened "+s.Long, w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return
}

//Pastebin handlers
func (env *thingEnv) APInewPaste(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "APInewPaste")
	log.Println("Paste request...")
	paste := r.Body
	buf := new(bytes.Buffer)
	buf.ReadFrom(paste)
	bpaste := buf.String()
	var name = ""
	params := mux.Vars(r)
	varname := params["name"]
	if varname != "" {
		name = varname
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
		Title:   name,
		Content: bpaste,
	}
	err := p.save(env)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
	fmt.Fprintln(w, getScheme(r)+r.Host+"/p/"+name)
}

func (env *thingEnv) APInewPasteForm(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "APInewPasteForm")
	err := r.ParseForm()
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}

	processCaptcha(w, r)

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
		Title:   title,
		Content: paste,
	}
	err = p.save(env)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
	http.Redirect(w, r, getScheme(r)+r.Host+"/p/"+title, 302)
}

//APIdeleteHandler deletes a given /{type}/{name}
func (env *thingEnv) APIdeleteHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "APIdeleteHandler")
	//Requests should come in on /api/delete/{type}/{name}
	params := mux.Vars(r)
	ftype := params["type"]
	fname := params["name"]
	jmsg := ftype + " " + fname

	db := env.getDB()
	defer env.closeDB()

	if ftype == "file" {
		err := db.Update(func(tx *bolt.Tx) error {
			log.Println(jmsg + " has been deleted")
			return tx.Bucket([]byte("Files")).Delete([]byte(fname))
		})
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println(err)
			return
		}
		fpath := viper.GetString("FileDir") + fname
		err = os.Remove(fpath)
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println(err)
			return
		}
		env.authState.SetFlash("Successfully deleted "+jmsg, w)
		http.Redirect(w, r, "/list", http.StatusSeeOther)
	} else if ftype == "image" {
		err := db.Update(func(tx *bolt.Tx) error {
			log.Println(jmsg + " has been deleted")
			return tx.Bucket([]byte("Images")).Delete([]byte(fname))
		})
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println(err)
			return
		}
		fpath := viper.GetString("ImgDir") + fname
		err = os.Remove(fpath)
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println(err)
			return
		}
		env.authState.SetFlash("Successfully deleted "+jmsg, w)
		http.Redirect(w, r, "/list", http.StatusSeeOther)
	} else if ftype == "paste" {
		err := db.Update(func(tx *bolt.Tx) error {
			log.Println(jmsg + " has been deleted")
			return tx.Bucket([]byte("Pastes")).Delete([]byte(fname))
		})
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println(err)
		}
		env.authState.SetFlash("Successfully deleted "+jmsg, w)
		http.Redirect(w, r, "/list", http.StatusSeeOther)
	} else if ftype == "shorturl" {
		err := db.Update(func(tx *bolt.Tx) error {
			log.Println(jmsg + " has been deleted")
			return tx.Bucket([]byte("Shorturls")).Delete([]byte(fname))
		})
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println(err)
		}
		env.authState.SetFlash("Successfully deleted "+jmsg, w)
		http.Redirect(w, r, "/list", http.StatusSeeOther)
	} else {
		env.authState.SetFlash("Failed to delete "+jmsg, w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (env *thingEnv) APIlgAction(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "APIlgAction")
	unsafeURL := r.PostFormValue("url")
	err := r.ParseForm()
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}

	url := bluemonday.StrictPolicy().Sanitize(unsafeURL)

	processCaptcha(w, r)

	if r.Form.Get("lg-action") == "ping" {
		//Ping stuff
		out, err := exec.Command("ping", "-c10", url).Output()
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println(err)
		}
		outs := string(out)
		title := "Pinging " + url
		p, err := loadPage(title, w, r)
		data := struct {
			Page    *Page
			Title   string
			Message string
		}{
			p,
			title,
			outs,
		}
		err = renderTemplate(env, w, "lg.tmpl", data)
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println(err)
		}
	} else if r.Form.Get("lg-action") == "mtr" {
		//MTR stuff
		out, err := exec.Command("mtr", "--report-wide", "-c10", url).Output()
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println(err)
		}
		outs := string(out)
		title := "MTR to " + url
		p, err := loadPage(title, w, r)
		data := struct {
			Page    *Page
			Title   string
			Message string
		}{
			p,
			title,
			outs,
		}
		err = renderTemplate(env, w, "lg.tmpl", data)
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println(err)
		}
	} else if r.Form.Get("lg-action") == "traceroute" {
		//Traceroute stuff
		out, err := exec.Command("traceroute", url).Output()
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println(err)
		}
		outs := string(out)
		title := "Traceroute to " + url
		p, err := loadPage(title, w, r)
		data := struct {
			Page    *Page
			Title   string
			Message string
		}{
			p,
			title,
			outs,
		}
		err = renderTemplate(env, w, "lg.tmpl", data)
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println(err)
		}
	} else {
		//If formvalue isn't MTR, Ping, or traceroute, this should be hit
		http.NotFound(w, r)
		return
	}
}

func (env *thingEnv) APInewRemoteImage(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "APInewRemoteImage")
	remoteURL := r.FormValue("remote-image")
	finURL := remoteURL

	if !strings.HasPrefix(remoteURL, "http") {
		log.Println("remoteURL does not contain a URL prefix, so adding http")
		log.Println(remoteURL)
		finURL = "http://" + remoteURL
	}
	fileURL, err := url.Parse(finURL)
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
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
	dlpath := viper.GetString("ImgDir")
	if r.FormValue("remote-image-name") != "" {
		fileName = sanitize.Name(r.FormValue("remote-image-name"))
		log.Println("custom remote image name: " + fileName)
	}
	file, err := os.Create(filepath.Join(dlpath, fileName))
	if err != nil {
		raven.CaptureError(err, nil)
		fmt.Println(err)
		env.authState.SetFlash("Failed to save remote image", w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		panic(err)
	}
	defer file.Close()
	check := http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}
	resp, err := check.Get(finURL)
	if err != nil {
		raven.CaptureError(err, nil)
		fmt.Println(err)
		env.authState.SetFlash("Failed to save remote image", w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		panic(err)
	}
	defer resp.Body.Close()
	fmt.Println(resp.Status)

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		raven.CaptureError(err, nil)
		env.authState.SetFlash("Failed to save remote image", w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		panic(err)
	}

	//BoltDB stuff
	imi := &Image{
		Created:   time.Now().Unix(),
		Filename:  fileName,
		RemoteURL: finURL,
	}
	err = imi.save(env)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		env.authState.SetFlash("Failed to save remote image", w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}

	env.authState.SetFlash("Successfully saved "+fileName+": https://"+viper.GetString("MainTLD")+"/i/"+fileName, w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (env *thingEnv) APInewImage(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "APInewImage")
	contentLength := r.ContentLength
	var reader io.Reader
	var f io.WriteCloser
	var err error
	var filename string
	path := viper.GetString("ImgDir")
	params := mux.Vars(r)
	formfilename := params["filename"]
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
				raven.CaptureError(err, nil)
				log.Printf("%s", err.Error())
				http.Error(w, err.Error(), 500)
				return
			}
			if n > _24K {
				file, err := ioutil.TempFile("./tmp/", "transfer-")
				if err != nil {
					raven.CaptureError(err, nil)
					log.Printf("%s", err.Error())
					http.Error(w, err.Error(), 500)
					return
				}
				defer file.Close()
				n, err = io.Copy(file, io.MultiReader(&b, f))
				if err != nil {
					raven.CaptureError(err, nil)
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
		filename = sanitize.Path(filepath.Base(formfilename))
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
			filename = sanitize.Name(r.FormValue("local-image-name"))
			log.Println("custom local image name: " + filename)
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
		contentType = mime.TypeByExtension(filepath.Ext(formfilename))
	} else {
		//log.Println("Content-type is " + contentType)
		err := r.ParseMultipartForm(_24K)
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println("ParseMultiform reader error")
			log.Println(err)
			return
		}
		file, handler, err := r.FormFile("file")
		filename = handler.Filename
		if r.FormValue("local-image-name") != "" {
			filename = sanitize.Name(r.FormValue("local-image-name"))
			log.Println("custom local image name: " + filename)
		}
		if err != nil {
			raven.CaptureError(err, nil)
			fmt.Println(err)
			return
		}
		defer file.Close()
		f, err := os.OpenFile(filepath.Join(path, filename), os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			raven.CaptureError(err, nil)
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

	// If this is a GIF, toss the GIF away and replace it with an MP4
	if filepath.Ext(filename) == ".gif" {
		log.Println("New gif detected; converting to mp4!")
		nameWithoutExt := filename[0 : len(filename)-len(filepath.Ext(".gif"))]
		path := viper.GetString("ImgDir")

		// ffmpeg -i doit.gif -vcodec h264 -y -pix_fmt yuv420p doit.mp4
		resize := exec.Command("/usr/bin/ffmpeg", "-i", filepath.Join(path, filename), "-vcodec", "h264", "-movflags", "faststart", "-y", "-pix_fmt", "yuv420p", "-vf", "scale='trunc(iw/2)*2:trunc(ih/2)*2'", filepath.Join(path, nameWithoutExt+".mp4"))
		err := resize.Run()
		if err != nil {
			raven.CaptureError(err, nil)
			log.Fatalln(resize.Args, err)
		}
		// After successful conversion, remove the originally uploaded gif
		err = os.Remove(filepath.Join(path, filename))
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println("Error removing gif after converting to mp4", filename, err)
		}
		filename = nameWithoutExt + ".mp4"
	}

	// w.Statuscode = 200

	// Check if we're uploading a screenshot
	ss := r.FormValue("screenshot")
	if ss == "on" {
		//BoltDB stuff
		sc := &Screenshot{
			Created:  time.Now().Unix(),
			Filename: filename,
		}
		err = sc.save(env)
		if err != nil {
			raven.CaptureError(err, nil)
			log.Println(err)
			env.authState.SetFlash("Failed to save screenshot", w)
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
		env.authState.SetFlash("Successfully saved screenshot "+filename+": https://"+viper.GetString("MainTLD")+"/i/"+filename, w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	//BoltDB stuff
	imi := &Image{
		Created:  time.Now().Unix(),
		Filename: filename,
	}
	err = imi.save(env)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		env.authState.SetFlash("Failed to save image", w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
	env.authState.SetFlash("Successfully saved image "+filename+": <a href=https://"+viper.GetString("MainTLD")+"/i/"+filename+"></a>", w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (env *thingEnv) Readme(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "Readme")
	name := "README"
	p, err := loadPage(name, w, r)
	if err != nil {
		raven.CaptureError(err, nil)
		http.NotFound(w, r)
		return
	}
	body, err := ioutil.ReadFile("./" + name + ".md")
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		return
	}
	//unsafe := blackfriday.MarkdownCommon(body)
	md := markdownRender(body)
	mdhtml := template.HTML(md)
	//html := bluemonday.UGCPolicy().SanitizeBytes(unsafe)
	data := struct {
		Page  *Page
		Title string
		MD    template.HTML
	}{
		p,
		name,
		mdhtml,
	}
	err = renderTemplate(env, w, "md.tmpl", data)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
	log.Println(name + " Page rendered!")
}

func (env *thingEnv) Changelog(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "Changelog")
	name := "CHANGELOG"
	p, err := loadPage(name, w, r)
	if err != nil {
		raven.CaptureError(err, nil)
		http.NotFound(w, r)
		return
	}
	body, err := ioutil.ReadFile("./" + name + ".md")
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
		return
	}
	//unsafe := blackfriday.MarkdownCommon(body)
	md := markdownRender(body)
	mdhtml := template.HTML(md)
	//html := bluemonday.UGCPolicy().SanitizeBytes(unsafe)
	data := struct {
		Page  *Page
		Title string
		MD    template.HTML
	}{
		p,
		name,
		mdhtml,
	}
	err = renderTemplate(env, w, "md.tmpl", data)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Println(err)
	}
	log.Println(name + " Page rendered!")
}
