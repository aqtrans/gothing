package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
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

	"git.jba.io/go/httputils"
	"git.jba.io/go/thing/things"
	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/kennygrant/sanitize"
	"github.com/microcosm-cc/bluemonday"
	"github.com/spf13/viper"
	//"jba.io/go/auth"
)

func (env *thingEnv) indexHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "indexHandler")
	title := "index"
	p, _ := loadMainPage(title, w, r)
	err := renderTemplate(env, w, "index.tmpl", p)
	if err != nil {
		errRedir(err, w)
		return
	}
}

func (env *thingEnv) helpHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "helpHandler")
	title := "Help"
	p, _ := loadMainPage(title, w, r)
	err := renderTemplate(env, w, "help.tmpl", p)
	if err != nil {
		errRedir(err, w)
		return
	}
}

func (env *thingEnv) loadGalleryPage(w http.ResponseWriter, r *http.Request) (*GalleryPage, error) {
	defer httputils.TimeTrack(time.Now(), "loadGalleryPage")
	page, perr := loadPage("Gallery", w, r)
	if perr != nil {
		return nil, perr
	}

	db := getDB()
	defer db.Close()

	var images []*things.Image
	//Lets try this with boltDB now!
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Images"))
		err := b.ForEach(func(k, v []byte) error {
			//fmt.Printf("key=%s, value=%s\n", k, v)
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
	return &GalleryPage{Page: page, Images: images}, nil
}

func (env *thingEnv) galleryHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "galleryHandler")
	l, err := env.loadGalleryPage(w, r)
	if err != nil {
		errRedir(err, w)
		return
	}

	err = renderTemplate(env, w, "gallery.tmpl", l)
	if err != nil {
		errRedir(err, w)
		return
	}
}

func (env *thingEnv) galleryEsgyHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "galleryEsgyHandler")
	l, err := env.loadGalleryPage(w, r)
	if err != nil {
		errRedir(err, w)
		return
	}

	err = renderTemplate(env, w, "gallery-esgy.tmpl", l)
	if err != nil {
		errRedir(err, w)
		return
	}
}

func (env *thingEnv) adminListHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "galleryListHandler")
	//title := "Admin List"
	l, err := env.loadGalleryPage(w, r)
	if err != nil {
		errRedir(err, w)
		return
	}
	err = renderTemplate(env, w, "admin_list.tmpl", l)
	if err != nil {
		errRedir(err, w)
		return
	}
}

func (env *thingEnv) adminHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "adminHandler")
	title := "Admin Panel"
	p, _ := loadMainPage(title, w, r)
	err := renderTemplate(env, w, "admin.tmpl", p)
	if err != nil {
		errRedir(err, w)
		return
	}
}

func (env *thingEnv) adminSignupHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "adminSignupHandler")
	title := "Admin Signup"
	p, _ := loadMainPage(title, w, r)
	err := renderTemplate(env, w, "admin_user.tmpl", p)
	if err != nil {
		errRedir(err, w)
		return
	}
}

func (env *thingEnv) adminUserPassHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "adminUserPassHandler")
	title := "Admin Password Change"
	p, _ := loadMainPage(title, w, r)
	err := renderTemplate(env, w, "admin_password.tmpl", p)
	if err != nil {
		errRedir(err, w)
		return
	}
}

func (env *thingEnv) signupPageHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "adminSignupHandler")
	title := "Signup"
	p, _ := loadMainPage(title, w, r)
	err := renderTemplate(env, w, "signup.tmpl", p)
	if err != nil {
		errRedir(err, w)
		return
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
		errRedir(err, w)
		return
	}
}

func (env *thingEnv) searchHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "searchHandler")
	params := mux.Vars(r)
	term := params["name"]
	sterm := regexp.MustCompile(term)

	file := &things.File{}
	paste := &things.Paste{}

	db := getDB()
	defer db.Close()

	//Lets try this with boltDB now!
	err := db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte("Pastes"))
		c.ForEach(func(k, v []byte) error {
			//fmt.Printf("key=%s, value=%s\n", k, v)
			err := json.Unmarshal(v, &paste)
			if err != nil {
				return err
			}
			plink := paste.Title
			pfull := paste.Title + paste.Content
			if sterm.MatchString(pfull) {
				fmt.Fprintln(w, plink)
			}
			return nil
		})
		d := tx.Bucket([]byte("Files"))
		err := d.ForEach(func(k, v []byte) error {
			//fmt.Printf("key=%s, value=%s\n", k, v)
			err := json.Unmarshal(v, &file)
			if err != nil {
				return err
			}
			flink := file.Filename
			if sterm.MatchString(flink) {
				fmt.Fprintln(w, flink)
			}
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		errRedir(err, w)
		return
	}

}

func (env *thingEnv) uploadPageHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "uploadPageHandler")
	title := "up"
	p, _ := loadMainPage(title, w, r)
	err := renderTemplate(env, w, "up.tmpl", p)
	if err != nil {
		errRedir(err, w)
		return
	}
}

func (env *thingEnv) uploadImagePageHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "uploadImagePageHandler")
	title := "upimg"
	p, _ := loadMainPage(title, w, r)
	err := renderTemplate(env, w, "upimg.tmpl", p)
	if err != nil {
		errRedir(err, w)
		return
	}
}

func (env *thingEnv) pastePageHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "pastePageHandler")
	title := "paste"
	p, _ := loadMainPage(title, w, r)
	err := r.ParseForm()
	if err != nil {
		errRedir(err, w)
		return
	}
	err = renderTemplate(env, w, "paste.tmpl", p)
	if err != nil {
		errRedir(err, w)
		return
	}

}

func (env *thingEnv) shortenPageHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "shortenPageHandler")
	title := "shorten"
	p, _ := loadMainPage(title, w, r)
	err := r.ParseForm()
	if err != nil {
		errRedir(err, w)
		return
	}
	err = renderTemplate(env, w, "shorten.tmpl", p)
	if err != nil {
		errRedir(err, w)
		return
	}

}

func (env *thingEnv) loginPageHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "loginPageHandler")
	title := "login"
	p, err := loadPage(title, w, r)
	if err != nil {
		errRedir(err, w)
		return
	}
	data := struct {
		Page  *Page
		Title string
	}{
		p,
		title,
	}
	err = renderTemplate(env, w, "login.tmpl", data)
	if err != nil {
		errRedir(err, w)
		return
	}
}

func (env *thingEnv) listHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "listHandler")
	l, err := env.loadListPage(w, r)
	if err != nil {
		errRedir(err, w)
		return
	}
	err = renderTemplate(env, w, "list.tmpl", l)
	if err != nil {
		errRedir(err, w)
		return
	}
}

//Short URL Handler
func (env *thingEnv) shortUrlHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "shortUrlHandler")

	params := mux.Vars(r)
	title := strings.ToLower(params["name"])

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

	shorturl := &things.Shorturl{}
	err := getThing(shorturl, title)
	if err != nil {
		if err == errNOSUCHTHING {
			http.Error(w, "404", http.StatusNotFound)
			return
		}
		errRedir(err, w)
	} else {
		destURL := shorturl.Long
		// If shorturl.Long begins with /, assume it is a file/image/screenshot to be served locally
		//    This is to replace the rest of the if/else now-commented out:
		if strings.HasPrefix(shorturl.Long, "/") {
			destURL = "//" + viper.GetString("MainTLD") + shorturl.Long
			//http.Redirect(w, r, "//"+viper.GetString("MainTLD")+shorturl.Long, 302)
		}
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

	paste := &things.Paste{}
	err := getThing(paste, title)
	if err != nil {
		if err == errNOSUCHTHING {
			http.Error(w, "404", http.StatusNotFound)
			return
		}
		errRedir(err, w)
		return
	}

	//No longer using BlueMonday or template.HTMLEscapeString because theyre too overzealous
	//I need '<' and '>' in tact for scripts and such

	//safe := template.HTMLEscapeString(paste.Content)
	//safe := sanitize.HTML(paste.Content)

	//safe := strings.Replace(paste.Content, "<script>", "< script >", -1)

	//safe := paste.Content

	updateHits(paste)

	// Bluemonday
	p := bluemonday.UGCPolicy()
	safe := p.Sanitize(paste.Content)

	fmt.Fprintf(w, "%s", safe)

}

func (env *thingEnv) downloadHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "downloadHandler")
	params := mux.Vars(r)
	name := params["name"]
	fpath := filepath.Join(viper.GetString("FileDir"), path.Base(name))
	//fpath := cfg.FileDir + path.Base(name)

	file := &things.File{}
	err := getThing(file, name)
	if err != nil {
		if err == errNOSUCHTHING {
			http.Error(w, "404", http.StatusNotFound)
			return
		}
		errRedir(err, w)
		return
	}
	updateHits(file)

	http.ServeFile(w, r, fpath)

}

func (env *thingEnv) downloadImageHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "downloadImageHandler")
	params := mux.Vars(r)
	name := params["name"]
	//fpath := cfg.ImgDir + path.Base(name)
	fpath := filepath.Join(viper.GetString("ImgDir"), path.Base(name))

	if name == "favicon.ico" {
		http.NotFound(w, r)
		return
	}
	if name == "favicon.png" {
		http.NotFound(w, r)
		return
	}

	extensions := []string{".mp4", ".webm", ".gif", ".jpg", ".jpeg", ".png"}
	//If this is extensionless, search for the proper file with the extension
	//  Note: Searching for mp4, webm first
	if filepath.Ext(name) == "" {
		for _, ext := range extensions {
			if _, err := os.Stat(fpath + ext); err == nil {
				name = name + ext
				//fpath = cfg.ImgDir + path.Base(name)
				fpath = filepath.Join(viper.GetString("ImgDir"), path.Base(name))
				break
			}
		}
	}

	// Try and ensure we have GIFs and MP4s for all images
	// If not, convert as necessary
	ext := filepath.Ext(name)

	switch ext {
	case ".mp4":
		//mp4BaseName := name[0 : len(name)-len(filepath.Ext(".mp4"))]
		//gifFullPath := filepath.Join(viper.GetString("ImgDir"), filenameWithoutExtension(name)+".gif")
		// If mp4 does not exist, check if a gif does
		if _, err := os.Stat(fpath); err != nil {
			if _, err := os.Stat(filepath.Join(viper.GetString("ImgDir"), filenameWithoutExtension(name)+".gif")); err != nil {
				break
			} else {
				err := gifToMP4(filenameWithoutExtension(name))
				if err != nil {
					log.Println("Failed to convert gifToMP4:", name, err)
					break
				} else {
					// Try and save newly-converted MP4, so it is served up below:
					err = saveThing(&things.Image{
						Created:  time.Now().Unix(),
						Filename: name,
					})
					if err != nil {
						log.Println("Error saving converted MP4:", err)
					}
				}
			}
		}
		// Check if the gif exists, if it doesn't, convert in a goroutine:
		if _, err := os.Stat(filepath.Join(viper.GetString("ImgDir"), filenameWithoutExtension(name)+".gif")); err != nil {
			go func() {
				log.Println("mp4 with no matching gif requested, converting ", name)
				err := mp4toGIF(filenameWithoutExtension(name))
				if err != nil {
					log.Println("Failed to convert mp4toGIF:", name, err)
					return
				}
				err = saveThing(&things.Image{
					Created:  time.Now().Unix(),
					Filename: filenameWithoutExtension(name) + ".gif",
				})
				if err != nil {
					log.Println("Error saving converted GIF:", err)
				}
			}()
		}
	case ".gif":
		//gifBaseName := name[0 : len(name)-len(filepath.Ext(".gif"))]
		//mp4FullPath := filepath.Join(viper.GetString("ImgDir"), filenameWithoutExtension(name)+".mp4")
		// If gif does not exist, check if an mp4 does
		if _, err := os.Stat(fpath); err != nil {
			if _, err := os.Stat(filepath.Join(viper.GetString("ImgDir"), filenameWithoutExtension(name)+".mp4")); err != nil {
				break
			} else {
				err := mp4toGIF(filenameWithoutExtension(name))
				if err != nil {
					log.Println("Failed to convert mp4toGIF:", name, err)
					break
				} else {
					// Try and save newly-converted GIF, so it is served up below:
					err = saveThing(&things.Image{
						Created:  time.Now().Unix(),
						Filename: name,
					})
					if err != nil {
						log.Println("Error saving converted GIF:", err)
					}
				}
			}
		}
		// Check if the mp4 exists, if it doesn't, convert in a goroutine:
		if _, err := os.Stat(filepath.Join(viper.GetString("ImgDir"), filenameWithoutExtension(name)+".mp4")); err != nil {
			go func() {
				log.Println("gif with no matching mp4 requested, converting ", name)
				err := gifToMP4(filenameWithoutExtension(name))
				if err != nil {
					log.Println("Failed to convert gifToMP4:", name, err)
					return
				}
				err = saveThing(&things.Image{
					Created:  time.Now().Unix(),
					Filename: filenameWithoutExtension(name) + ".mp4",
				})
				if err != nil {
					log.Println("Error saving converted MP4:", err)
				}
			}()
		}
	}

	//Attempt to increment file hit counter...
	image := &things.Image{}
	err := getThing(image, name)
	if err != nil {
		if err == errNOSUCHTHING {
			http.Error(w, "404", http.StatusNotFound)
			return
		}
		errRedir(err, w)
		return
	}
	updateHits(image)

	/*
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
	*/

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
	defer httputils.TimeTrack(time.Now(), "imageThumbHandler")
	params := mux.Vars(r)
	name := params["name"]
	fpath := filepath.Join(viper.GetString("ImgDir"), path.Base(strings.TrimSuffix(name, ".png")))
	thumbPath := filepath.Join(viper.GetString("ThumbDir"), path.Base(name))

	//Check to see if the large image already exists
	//If so, serve it directly
	if _, err := os.Stat(thumbPath); err == nil {
		http.ServeFile(w, r, filepath.Join(viper.GetString("ThumbDir"), path.Base(name)))
	} else {
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
		errRedir(err, w)
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
	serveContent(w, r, viper.GetString("ImgDir"), name)

}

// imageBigHandler uses a weird CSS trick to make the images really big
func imageBigHandler(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "imageBigHandler")
	params := mux.Vars(r)
	name := params["name"]

	// Try and intercept GIF requests since they should all be MP4s now
	if filepath.Ext(name) == ".gif" {
		nameWithoutExt := filenameWithoutExtension(name)
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
		errRedir(err, w)
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
		errRedir(err, w)
		return
	}

	body, err := ioutil.ReadFile("./md/" + name + ".md")
	if err != nil {
		errRedir(err, w)
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
		errRedir(err, w)
		return
	}
}

func (env *thingEnv) APInewRemoteFile(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "APInewRemoteFile")

	remoteURL := r.FormValue("remote")
	finURL := remoteURL
	if !strings.HasPrefix(remoteURL, "http") {
		finURL = "http://" + remoteURL
	}
	fileURL, err := url.Parse(finURL)
	if err != nil {
		errRedir(err, w)
		return
	}
	path := fileURL.Path
	segments := strings.Split(path, "/")
	fileName := segments[len(segments)-1]

	dlpath := viper.GetString("FileDir")
	if r.FormValue("remote-file-name") != "" {
		fileName = sanitize.Name(r.FormValue("remote-file-name"))
	}
	file, err := os.Create(filepath.Join(dlpath, fileName))
	if err != nil {
		errRedir(err, w)
		return
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
		errRedir(err, w)
		return
	}
	defer resp.Body.Close()
	fmt.Println(resp.Status)

	size, err := io.Copy(file, resp.Body)
	if err != nil {
		errRedir(err, w)
		return
	}

	//BoltDB stuff
	fi := &things.File{
		Created:   time.Now().Unix(),
		Filename:  fileName,
		RemoteURL: finURL,
	}
	err = saveThing(fi)
	if err != nil {
		errRedir(err, w)
		return
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
	var fi *things.File
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

	//Remote File Uploads
	if uptype == "remote" {
		remoteURL := r.FormValue("remote")
		finURL := remoteURL
		if !strings.HasPrefix(remoteURL, "http") {
			finURL = "http://" + remoteURL
		}
		fileURL, err := url.Parse(finURL)
		if err != nil {
			errRedir(err, w)
			return
		}
		//path := fileURL.Path
		segments := strings.Split(fileURL.Path, "/")
		filename = segments[len(segments)-1]

		//dlpath := cfg.FileDir
		if r.FormValue("remote-file-name") != "" {
			filename = sanitize.Name(r.FormValue("remote-file-name"))
		}
		file, err := os.Create(filepath.Join(path, filename))
		if err != nil {
			errRedir(err, w)
			return
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
			errRedir(err, w)
			return
		}
		defer resp.Body.Close()
		fmt.Println(resp.Status)

		size, err := io.Copy(file, resp.Body)
		if err != nil {
			errRedir(err, w)
			return
		}

		//BoltDB stuff
		fi = &things.File{
			Created:   time.Now().Unix(),
			Filename:  filename,
			RemoteURL: finURL,
		}

		//fmt.Printf("%s with %v bytes downloaded", fileName, size)
		//fmt.Fprintf(w, "%s with %v bytes downloaded from %s", fileName, size, finURL)
		fmt.Printf("%s with %v bytes downloaded from %s", filename, size, finURL)
	} else if uptype == "cli" {
		//Content-type blank, so this should be a CLI upload...
		reader = r.Body
		if contentLength == -1 {
			var err error
			var f io.Reader
			f = reader
			var b bytes.Buffer
			n, err := io.CopyN(&b, f, _24K+1)
			if err != nil && err != io.EOF {
				errRedir(err, w)
				return
			}
			if n > _24K {
				file, err := ioutil.TempFile("./tmp/", "transfer-")
				if err != nil {
					errRedir(err, w)
					return
				}
				defer file.Close()
				n, err = io.Copy(file, io.MultiReader(&b, f))
				if err != nil {
					errRedir(err, w)
					return
				}
				reader, err = os.Open(file.Name())
				if err != nil {
					errRedir(err, w)
					return
				}
			} else {
				reader = bytes.NewReader(b.Bytes())
			}
			contentLength = n
		}
		filename = sanitize.Path(filepath.Base(name))
		if filename == "." {
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
		}

		if f, err = os.OpenFile(filepath.Join(path, filename), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600); err != nil {
			errRedir(err, w)
			return
		}
		defer f.Close()
		if _, err = io.Copy(f, reader); err != nil {
			return
		}
		contentType = mime.TypeByExtension(filepath.Ext(name))
		//BoltDB stuff
		fi = &things.File{
			Created:  time.Now().Unix(),
			Filename: filename,
		}
	} else if uptype == "form" {
		err := r.ParseMultipartForm(_24K)
		if err != nil {
			errRedir(err, w)
			return
		}
		file, handler, err := r.FormFile("file")
		filename = handler.Filename
		defer file.Close()
		if err != nil {
			errRedir(err, w)
			return
		}
		if r.FormValue("local-file-name") != "" {
			filename = sanitize.Name(r.FormValue("local-file-name"))
		}

		f, err := os.OpenFile(filepath.Join(path, filename), os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			errRedir(err, w)
			return
		}
		defer f.Close()
		_, err = io.Copy(f, file)
		if err != nil {
			errRedir(err, w)
			return
		}

		//BoltDB stuff
		fi = &things.File{
			Created:  time.Now().Unix(),
			Filename: filename,
		}
	}

	err = saveThing(fi)
	if err != nil {
		errRedir(err, w)
		return
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
		errRedir(err, w)
		return
	}

	short := strings.ToLower(r.PostFormValue("short"))
	long := r.PostFormValue("long")

	/*
		short := bluemonday.StrictPolicy().Sanitize(unsafeShort)

		longPolicy := bluemonday.StrictPolicy()
		longPolicy.AllowStandardURLs()
		long := longPolicy.Sanitize(unsafeLong)
	*/

	s := &things.Shorturl{
		Created: time.Now().Unix(),
		Short:   short,
		Long:    long,
	}

	err = saveThing(s)
	if err != nil {
		errRedir(err, w)
		return
	}

	env.authState.SetFlash("Successfully shortened "+s.Long, w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return
}

//Pastebin handlers
func (env *thingEnv) APInewPaste(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "APInewPaste")
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
	p := &things.Paste{
		Created: time.Now().Unix(),
		Title:   name,
		Content: bpaste,
	}
	err := saveThing(p)
	if err != nil {
		errRedir(err, w)
		return
	}
	fmt.Fprintln(w, getScheme(r)+r.Host+"/p/"+name)
}

func (env *thingEnv) APInewPasteForm(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "APInewPasteForm")
	err := r.ParseForm()
	if err != nil {
		errRedir(err, w)
		return
	}

	success, err := env.captcha.Verify(r.FormValue("g-recaptcha-response"), r.RemoteAddr)
	if err != nil {
		errRedir(err, w)
		return
	}
	if !success {
		env.authState.SetFlash("Error verifying reCAPTCHA", w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	title := r.PostFormValue("title")
	if title == "" {
		dictionary := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
		var bytes = make([]byte, 4)
		rand.Read(bytes)
		for k, v := range bytes {
			bytes[k] = dictionary[v%byte(len(dictionary))]
		}
		title = string(bytes)
	}
	paste := r.PostFormValue("paste")
	p := &things.Paste{
		Created: time.Now().Unix(),
		Title:   title,
		Content: paste,
	}
	err = saveThing(p)
	if err != nil {
		errRedir(err, w)
		return
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

	db := getDB()
	defer db.Close()

	if ftype == "file" {
		err := db.Update(func(tx *bolt.Tx) error {
			//log.Println(jmsg + " has been deleted")
			return tx.Bucket([]byte("Files")).Delete([]byte(fname))
		})
		if err != nil {
			errRedir(err, w)
			return
		}
		fpath := viper.GetString("FileDir") + fname
		err = os.Remove(fpath)
		if err != nil {
			errRedir(err, w)
			return
		}
		env.authState.SetFlash("Successfully deleted "+jmsg, w)
		http.Redirect(w, r, "/list", http.StatusSeeOther)
	} else if ftype == "image" {
		err := db.Update(func(tx *bolt.Tx) error {
			//log.Println(jmsg + " has been deleted")
			return tx.Bucket([]byte("Images")).Delete([]byte(fname))
		})
		if err != nil {
			errRedir(err, w)
			return
		}
		fpath := viper.GetString("ImgDir") + fname
		err = os.Remove(fpath)
		if err != nil {
			errRedir(err, w)
			return
		}
		env.authState.SetFlash("Successfully deleted "+jmsg, w)
		http.Redirect(w, r, "/list", http.StatusSeeOther)
	} else if ftype == "paste" {
		err := db.Update(func(tx *bolt.Tx) error {
			//log.Println(jmsg + " has been deleted")
			return tx.Bucket([]byte("Pastes")).Delete([]byte(fname))
		})
		if err != nil {
			errRedir(err, w)
			return
		}
		env.authState.SetFlash("Successfully deleted "+jmsg, w)
		http.Redirect(w, r, "/list", http.StatusSeeOther)
	} else if ftype == "shorturl" {
		err := db.Update(func(tx *bolt.Tx) error {
			//log.Println(jmsg + " has been deleted")
			return tx.Bucket([]byte("Shorturls")).Delete([]byte(fname))
		})
		if err != nil {
			errRedir(err, w)
			return
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
		errRedir(err, w)
		return
	}

	url := bluemonday.StrictPolicy().Sanitize(unsafeURL)

	success, err := env.captcha.Verify(r.FormValue("g-recaptcha-response"), r.RemoteAddr)
	if err != nil {
		errRedir(err, w)
		return
	}
	if !success {
		env.authState.SetFlash("Error verifying reCAPTCHA", w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if r.Form.Get("lg-action") == "ping" {
		//Ping stuff
		out, err := exec.Command("ping", "-c10", url).Output()
		if err != nil {
			errRedir(err, w)
			return
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
			errRedir(err, w)
			return
		}
	} else if r.Form.Get("lg-action") == "mtr" {
		//MTR stuff
		out, err := exec.Command("mtr", "--report-wide", "-c10", url).Output()
		if err != nil {
			errRedir(err, w)
			return
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
			errRedir(err, w)
			return
		}
	} else if r.Form.Get("lg-action") == "traceroute" {
		//Traceroute stuff
		out, err := exec.Command("traceroute", url).Output()
		if err != nil {
			errRedir(err, w)
			return
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
			errRedir(err, w)
			return
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
		finURL = "http://" + remoteURL
	}
	fileURL, err := url.Parse(finURL)
	if err != nil {
		errRedir(err, w)
		return
	}
	path := fileURL.Path
	segments := strings.Split(path, "/")
	fileName := segments[len(segments)-1]

	dlpath := viper.GetString("ImgDir")
	if r.FormValue("remote-image-name") != "" {
		fileName = sanitize.Name(r.FormValue("remote-image-name"))
	}
	file, err := os.Create(filepath.Join(dlpath, fileName))
	if err != nil {
		errRedir(err, w)
		return
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
		errRedir(err, w)
		return
	}
	defer resp.Body.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		errRedir(err, w)
		return
	}

	//BoltDB stuff
	imi := &things.Image{
		Created:   time.Now().Unix(),
		Filename:  fileName,
		RemoteURL: finURL,
	}
	err = saveThing(imi)
	if err != nil {
		errRedir(err, w)
		return
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
		//Content-type blank, so this should be a CLI upload...
		reader = r.Body
		if contentLength == -1 {
			var err error
			var f io.Reader
			f = reader
			var b bytes.Buffer
			n, err := io.CopyN(&b, f, _24K+1)
			if err != nil && err != io.EOF {
				errRedir(err, w)
				return
			}
			if n > _24K {
				file, err := ioutil.TempFile("./tmp/", "transfer-")
				if err != nil {
					errRedir(err, w)
					return
				}
				defer file.Close()
				n, err = io.Copy(file, io.MultiReader(&b, f))
				if err != nil {
					errRedir(err, w)
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
		}

		if f, err = os.OpenFile(filepath.Join(path, filename), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600); err != nil {
			errRedir(err, w)
			return
		}
		defer f.Close()
		if _, err = io.Copy(f, reader); err != nil {
			errRedir(err, w)
			return
		}
		contentType = mime.TypeByExtension(filepath.Ext(formfilename))
	} else {
		err := r.ParseMultipartForm(_24K)
		if err != nil {
			errRedir(err, w)
			return
		}
		file, handler, err := r.FormFile("file")
		filename = handler.Filename
		if r.FormValue("local-image-name") != "" {
			filename = sanitize.Name(r.FormValue("local-image-name"))
		}
		if err != nil {
			errRedir(err, w)
			return
		}
		defer file.Close()
		f, err := os.OpenFile(filepath.Join(path, filename), os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			errRedir(err, w)
			return
		}
		defer f.Close()

		_, err = io.Copy(f, file)
		if err != nil {
			errRedir(err, w)
			return
		}

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

	// If this is a GIF, convert and save an MP4 copy
	if filepath.Ext(filename) == ".gif" {
		go func() {
			nameWithoutExt := filenameWithoutExtension(filename)

			err := gifToMP4(nameWithoutExt)
			if err != nil {
				log.Println("gifToMP4:", err)
				return
			}

			mp4Image := &things.Image{
				Created:  time.Now().Unix(),
				Filename: nameWithoutExt + ".mp4",
			}
			err = saveThing(mp4Image)
			if err != nil {
				log.Println("Error saving converted MP4:", err)
			}
		}()

		/*
			// After successful conversion, remove the originally uploaded gif
			err = os.Remove(filepath.Join(path, filename))
			if err != nil {
				log.Println("Error removing gif after converting to mp4", filename, err)
				errRedir(err, w)
				return
			}
			filename = nameWithoutExt + ".mp4"
		*/
	}

	// w.Statuscode = 200

	// Check if we're uploading a screenshot
	ss := r.FormValue("screenshot")
	if ss == "on" {
		//BoltDB stuff
		sc := &things.Screenshot{
			Created:  time.Now().Unix(),
			Filename: filename,
		}
		err = saveThing(sc)
		if err != nil {
			errRedir(err, w)
			return
		}
		env.authState.SetFlash("Successfully saved screenshot "+filename+": https://"+viper.GetString("MainTLD")+"/i/"+filename, w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	//BoltDB stuff
	imi := &things.Image{
		Created:  time.Now().Unix(),
		Filename: filename,
	}
	err = saveThing(imi)
	if err != nil {
		errRedir(err, w)
		return
	}
	env.authState.SetFlash("Successfully saved image "+filename+": <a href=https://"+viper.GetString("MainTLD")+"/i/"+filename+"></a>", w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (env *thingEnv) Readme(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "Readme")
	name := "README"
	p, err := loadPage(name, w, r)
	if err != nil {
		errRedir(err, w)
		return
	}
	body, err := ioutil.ReadFile("./" + name + ".md")
	if err != nil {
		errRedir(err, w)
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
		errRedir(err, w)
		return
	}
}

func (env *thingEnv) Changelog(w http.ResponseWriter, r *http.Request) {
	defer httputils.TimeTrack(time.Now(), "Changelog")
	name := "CHANGELOG"
	p, err := loadPage(name, w, r)
	if err != nil {
		errRedir(err, w)
		return
	}
	body, err := ioutil.ReadFile("./" + name + ".md")
	if err != nil {
		errRedir(err, w)
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
		errRedir(err, w)
		return
	}
}

func filenameWithoutExtension(fn string) string {
	return strings.TrimSuffix(fn, path.Ext(fn))
}

func gifToMP4(baseFilename string) error {
	// ffmpeg -i doit.gif -vcodec h264 -y -pix_fmt yuv420p doit.mp4
	// Per https://engineering.giphy.com/how-to-make-gifs-with-ffmpeg/: ffmpeg -i doit.gif -filter_complex "[0:v]fps=15" -vsync 0 -f mp4 -pix_fmt yuv420p 321.mp4
	path := viper.GetString("ImgDir")
	resize := exec.Command("/usr/bin/ffmpeg", "-i", filepath.Join(path, baseFilename+".gif"), "-filter_complex", "[0:v]fps=15", "-vsync", "0", "-f", "mp4", "-pix_fmt", "yuv420p", filepath.Join(path, baseFilename+".mp4"))
	err := resize.Run()
	if err != nil {
		return fmt.Errorf("Error converting GIF to MP4. args: %v Err: %v", resize.Args, err)
	}
	return nil
}

func mp4toGIF(baseFilename string) error {
	// Per https://engineering.giphy.com/how-to-make-gifs-with-ffmpeg/: ffmpeg -i doit.mp4 -filter_complex "[0:v] fps=12,scale=480:-1,split [a][b];[a] palettegen [p];[b][p] paletteuse" doit.gif
	path := viper.GetString("ImgDir")
	mp4Filename := baseFilename + ".mp4"
	resize := exec.Command("/usr/bin/ffmpeg", "-i", filepath.Join(path, mp4Filename), "-filter_complex", "[0:v]fps=15,scale=480:-1,split[a][b];[a]palettegen[p];[b][p]paletteuse", filepath.Join(path, baseFilename+".gif"))
	err := resize.Run()
	if err != nil {
		return fmt.Errorf("Error converting MP4 to GIF. args: %v Err: %v", resize.Args, err)
	}
	return nil
}
