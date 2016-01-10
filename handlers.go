package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	"github.com/kennygrant/sanitize"
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
)

func indexHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "indexHandler")
	title := "index"
	p, _ := loadMainPage(title, r)
	err := renderTemplate(w, "index.tmpl", p)
	if err != nil {
		log.Println(err)
	}
}

func loadGalleryPage(r *http.Request) (*GalleryPage, error) {
	defer timeTrack(time.Now(), "loadGalleryPage")
	page, perr := loadPage("Gallery", r)
	if perr != nil {
		log.Println(perr)
	}

	var images []*Image
	//Lets try this with boltDB now!
	db.View(func(tx *bolt.Tx) error {
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
	return &GalleryPage{Page: page, Images: images}, nil
}

func galleryHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "galleryHandler")
	l, err := loadGalleryPage(r)
	if err != nil {
		log.Println(err)
	}

	err = renderTemplate(w, "gallery.tmpl", l)
	if err != nil {
		log.Println(err)
	}
}

func galleryEsgyHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "galleryEsgyHandler")
	l, err := loadGalleryPage(r)
	if err != nil {
		log.Println(err)
	}

	err = renderTemplate(w, "gallery-esgy.tmpl", l)
	if err != nil {
		log.Println(err)
	}
}

func galleryListHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "galleryListHandler")
	l, err := loadGalleryPage(r)
	if err != nil {
		log.Println(err)
	}

	err = renderTemplate(w, "admin-list.tmpl", l)
	if err != nil {
		log.Println(err)
	}
}

func lgHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "lgHandler")
	title := "lg"
	p, err := loadPage(title, r)
	data := struct {
		Page    *Page
		Title   string
		Message string
	}{
		p,
		title,
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
	term := vars["name"]
	sterm := regexp.MustCompile(term)

	file := &File{}
	paste := &Paste{}

	//Lets try this with boltDB now!
	db.View(func(tx *bolt.Tx) error {
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

func uploadPageHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "uploadPageHandler")
	title := "up"
	p, _ := loadMainPage(title, r)
	err := renderTemplate(w, "up.tmpl", p)
	if err != nil {
		log.Println(err)
	}
}

func uploadImagePageHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "uploadImagePageHandler")
	title := "upimg"
	p, _ := loadMainPage(title, r)
	err := renderTemplate(w, "upimg.tmpl", p)
	if err != nil {
		log.Println(err)
	}
}

func pastePageHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "pastePageHandler")
	title := "paste"
	p, _ := loadMainPage(title, r)
	err := renderTemplate(w, "paste.tmpl", p)
	r.ParseForm()
	//log.Println(r.Form)
	if err != nil {
		log.Println(err)
	}
}

func shortenPageHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "shortenPageHandler")
	title := "shorten"
	p, _ := loadMainPage(title, r)
	err := renderTemplate(w, "shorten.tmpl", p)
	r.ParseForm()
	//log.Println(r.Form)
	if err != nil {
		log.Println(err)
	}
}

func loginPageHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "loginPageHandler")
	title := "login"
	p, err := loadPage(title, r)
	data := struct {
		Page  *Page
		Title string
	}{
		p,
		title,
	}
	err = renderTemplate(w, "login.tmpl", data)
	if err != nil {
		log.Println(err)
		return
	}
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "listHandler")
	l, err := loadListPage(r)
	if err != nil {
		log.Println(err)
	}
	err = renderTemplate(w, "list.tmpl", l)
	if err != nil {
		log.Println(err)
	}
}

//Short URL Handlers
func shortUrlHandler(w http.ResponseWriter, r *http.Request) {

	defer timeTrack(time.Now(), "shortUrlHandler")

	shorturl := &Shorturl{}
	vars := mux.Vars(r)
	title := vars["name"]
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
	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Shorturls"))
		v := b.Get([]byte(title))
		//Because BoldDB's View() doesn't return an error if there's no key found, just throw a 404 on nil
		//After JSON Unmarshal, Content should be in paste.Content field
		if v == nil {
			http.Error(w, "Error 400 - No such domain at this address", 400)
			err := errors.New(title + "No Such Short URL")
			return err
			//log.Println(err)
		}
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
                log.Println("Serving " + shorturl.Long + " file directly")
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
                log.Println("Serving " + shorturl.Long + " file directly")
                http.ServeFile(w, r, cfg.ImgDir+fileName)
            }
        }
        http.Redirect(w, r, shorturl.Long, 302)

        s := &Shorturl{
            Created: shorturl.Created,
            Short:   shorturl.Short,
            Long:    shorturl.Long,
            Hits:    count,
        }
        encoded, err := json.Marshal(s)

        //return nil
        return b.Put([]byte(title), encoded)
	})
	if err != nil {
		log.Println(err)
	}
}

func pasteHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "pasteHandler")
	vars := mux.Vars(r)
	title := vars["name"]
	paste := &Paste{}
	err := db.View(func(tx *bolt.Tx) error {
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
		log.Println(err)
	}
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "downloadHandler")
	vars := mux.Vars(r)
	name := vars["name"]
	fpath := cfg.FileDir + path.Base(name)

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

func downloadImageHandler(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "downloadImageHandler")
	vars := mux.Vars(r)
	name := vars["name"]
	fpath := cfg.ImgDir + path.Base(name)
    
    if name == "favicon.ico" {
        log.Println("omg1")
        http.NotFound(w, r)
        return
    }
    if name == "favicon.png" {
        log.Println("omg2")
        http.NotFound(w, r)
        return
    }
    
	extensions := []string{".webm", ".gif", ".jpg", ".jpeg", ".png"}
	//If this is extensionless, search for the proper file with the extension
	if filepath.Ext(name) == "" {
		//log.Println("NO EXTENSION FOUND OMG")
		for _, ext := range extensions {
			if _, err := os.Stat(fpath + ext); err == nil {
				name = name + ext
				fpath = cfg.ImgDir + path.Base(name)
                log.Println(name + fpath)
				break
			} else {
                log.Println(err)
            }
		}
	}

	//Attempt to increment file hit counter...
	image := &Image{}
	db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Images"))
		v := b.Get([]byte(name))
		//If there is no existing key, do not do a thing
		if v == nil {
			http.NotFound(w, r)
            log.Println("omg3")
			return nil
		}
		err := json.Unmarshal(v, &image)
		if err != nil {
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

	//If this is a webm file, serve it so it acts like a GIF
	if filepath.Ext(name) == ".webm" {
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
	vars := mux.Vars(r)
	name := vars["name"]
	fpath := cfg.ImgDir + path.Base(strings.TrimSuffix(name, ".png"))
	thumbPath := cfg.ThumbDir + path.Base(name)

	//log.Println("name:"+ name)
	//log.Println("fpath:"+ fpath)
	//log.Println("thumbpath:"+thumbPath)

	//Check to see if the large image already exists
	//If so, serve it directly
	if _, err := os.Stat(thumbPath); err == nil {
		log.Println("Pre-existing thumbnail already found, serving it...")
		http.ServeFile(w, r, cfg.ThumbDir+path.Base(name))
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

		http.ServeFile(w, r, cfg.ThumbDir+path.Base(name))
	}

}

func imageDirectHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	fpath := cfg.ImgDir + path.Base(name)
	http.ServeFile(w, r, fpath)
}

//Resizes all images using gifsicle command, due to image.resize failing at animated GIFs
//Images are dumped to ./tmp/ for now, probably want to fix this but I'm unsure where to put them
func imageBigHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	smallPath := cfg.ImgDir + path.Base(name)
	bigPath := cfg.GifDir + path.Base(name)

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
		resize := exec.Command("/usr/bin/gifsicle", "--conserve-memory", "--colors", "256", "--resize", "2000x_", smallPath, "-o", bigPath)
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
	smallPath := cfg.ImgDir + path.Base(name)
	bigPath := cfg.GifDir + path.Base(name)

	//Check to see if the large image already exists
	//If so, serve it directly
	if _, err := os.Stat(bigPath); err == nil {
		log.Println("Pre-existing BIG gif already found, serving it...")
		return
	}
    log.Println("BIG gif not found. Running gifsicle...")
    file, err := os.Open(smallPath)
    if err != nil {
        log.Println(err)
        return
    }
    file.Close()
    //gifsicle --conserve-memory --colors 256 --resize 2000x_ ./up-imgs/groove_fox.gif -o ./tmp/BIG-groove_fox.gif
    resize := exec.Command("/usr/bin/gifsicle", "--conserve-memory", "--colors", "256", "--resize", "2000x_", smallPath, "-o", bigPath)
    err = resize.Run()
    if err != nil {
        log.Println(err)
    }
    log.Println(name + " BIG GIF has been saved.")
}

func viewMarkdownHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	p, err := loadPage(name, r)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	body, err := ioutil.ReadFile("./md/" + name + ".md")
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
		Page  *Page
		Title string
		MD    template.HTML
	}{
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

func APInewRemoteFile(w http.ResponseWriter, r *http.Request) {
	remoteURL := r.FormValue("remote")
	finURL := remoteURL
	if !strings.HasPrefix(remoteURL, "http") {
		log.Println("remoteURL does not contain a URL prefix, so adding http")
		finURL = "http://" + remoteURL
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
		fileName = sanitize.Name(r.FormValue("remote-file-name"))
		log.Println("custom remote file name: " + fileName)
	}
	file, err := os.Create(filepath.Join(dlpath, fileName))
	if err != nil {
		fmt.Println(err)
        WriteJ(w, "", false)
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
		fmt.Println(err)
        WriteJ(w, "", false)
		panic(err)
	}
	defer resp.Body.Close()
	fmt.Println(resp.Status)

	size, err := io.Copy(file, resp.Body)
	if err != nil {
        WriteJ(w, "", false)
		panic(err)
	}

	//BoltDB stuff
	fi := &File{
		Created:   time.Now().Unix(),
		Filename:  fileName,
		RemoteURL: finURL,
	}
	err = fi.save()
	if err != nil {
		log.Println(err)
		WriteJ(w, "", false)
	}

	//fmt.Printf("%s with %v bytes downloaded", fileName, size)
	//fmt.Fprintf(w, "%s with %v bytes downloaded from %s", fileName, size, finURL)
	fmt.Printf("%s with %v bytes downloaded from %s", fileName, size, finURL)

	WriteJ(w, fileName, true)
}

func APInewFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
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
		if !strings.HasPrefix(remoteURL, "http") {
			log.Println("remoteURL does not contain a URL prefix, so adding http")
			finURL = "http://" + remoteURL
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
			filename = sanitize.Name(r.FormValue("remote-file-name"))
			log.Println("custom remote file name: " + filename)
		}
		file, err := os.Create(filepath.Join(path, filename))
		if err != nil {
            WriteJ(w, "", false)
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
            WriteJ(w, "", false)
			fmt.Println(err)
			panic(err)
		}
		defer resp.Body.Close()
		fmt.Println(resp.Status)

		size, err := io.Copy(file, resp.Body)
		if err != nil {
            WriteJ(w, "", false)
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
			log.Println("ParseMultiform reader error")
			log.Println(err)
			WriteJ(w, "", false)
			return
		}
		file, handler, err := r.FormFile("file")
		filename = handler.Filename
		defer file.Close()
		if err != nil {
			fmt.Println(err)
			WriteJ(w, "", false)
		}
		if r.FormValue("local-file-name") != "" {
			filename = sanitize.Name(r.FormValue("local-file-name"))
			log.Println("custom local file name: " + filename)
		}

		f, err := os.OpenFile(filepath.Join(path, filename), os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			fmt.Println(err)
			WriteJ(w, "", false)
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

	err = fi.save()
	if err != nil {
		log.Println(err)
		WriteJ(w, "", false)
	}

	if uptype == "cli" {
		fmt.Fprintf(w, "http://go.jba.io/d/"+filename)
	} else {
		WriteJ(w, filename, true)
	}
}

func APInewShortUrlForm(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "APInewShortUrlForm")
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
		WriteJ(w, "", false)
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
		Short:   short,
		Long:    long,
	}

	/*
		Created string
		Short 	string
		Long 	string
	*/

	err = s.save()
	if err != nil {
		log.Println(err)
		WriteJ(w, "", false)
	}
	//log.Println("Short: " + s.Short)
	//log.Println("Long: " + s.Long)

	WriteJ(w, s.Short, true)

}

//Pastebin handlers
func APInewPaste(w http.ResponseWriter, r *http.Request) {
	defer timeTrack(time.Now(), "APInewPaste")
	log.Println("Paste request...")
	paste := r.Body
	buf := new(bytes.Buffer)
	buf.ReadFrom(paste)
	bpaste := buf.String()
	var name = ""
	vars := mux.Vars(r)
	varname := vars["name"]
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
	err := p.save()
	if err != nil {
		log.Println(err)
	}
	fmt.Fprintln(w, getScheme(r)+r.Host+"/p/"+name)
}

func APInewPasteForm(w http.ResponseWriter, r *http.Request) {
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
		Title:   title,
		Content: paste,
	}
	err = p.save()
	if err != nil {
		log.Println(err)
	}
	http.Redirect(w, r, getScheme(r)+r.Host+"/p/"+title, 302)
}

//Delete stuff
func APIdeleteHandler(w http.ResponseWriter, r *http.Request) {
	//Requests should come in on /api/delete/{type}/{name}
	vars := mux.Vars(r)
	ftype := vars["type"]
	fname := vars["name"]
	if ftype == "file" {
		err := db.Update(func(tx *bolt.Tx) error {
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
		WriteJ(w, fname, true)
	} else if ftype == "image" {
		err := db.Update(func(tx *bolt.Tx) error {
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
		WriteJ(w, fname, true)
	} else if ftype == "paste" {
		err := db.Update(func(tx *bolt.Tx) error {
			log.Println(ftype + " " + fname + " has been deleted")
			return tx.Bucket([]byte("Pastes")).Delete([]byte(fname))
		})
		if err != nil {
			log.Println(err)
		}
		WriteJ(w, fname, true)
	} else if ftype == "shorturl" {
		err := db.Update(func(tx *bolt.Tx) error {
			log.Println(ftype + " " + fname + " has been deleted")
			return tx.Bucket([]byte("Shorturls")).Delete([]byte(fname))
		})
		if err != nil {
			log.Println(err)
		}
		WriteJ(w, fname, true)
	} else {
		WriteJ(w, "", false)
	}
}

func APIlgAction(w http.ResponseWriter, r *http.Request) {
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
		p, err := loadPage(title, r)
		data := struct {
			Page    *Page
			Title   string
			Message string
		}{
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
		p, err := loadPage(title, r)
		data := struct {
			Page    *Page
			Title   string
			Message string
		}{
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
		p, err := loadPage(title, r)
		data := struct {
			Page    *Page
			Title   string
			Message string
		}{
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

func APInewRemoteImage(w http.ResponseWriter, r *http.Request) {
	remoteURL := r.FormValue("remote-image")
	finURL := remoteURL
	if !strings.HasPrefix(remoteURL, "http") {
		log.Println("remoteURL does not contain a URL prefix, so adding http")
		log.Println(remoteURL)
		finURL = "http://" + remoteURL
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
		fileName = sanitize.Name(r.FormValue("remote-image-name"))
		log.Println("custom remote image name: " + fileName)
	}
	file, err := os.Create(filepath.Join(dlpath, fileName))
	if err != nil {
		fmt.Println(err)
		WriteJ(w, "", false)
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
		fmt.Println(err)
		WriteJ(w, "", false)
		panic(err)
	}
	defer resp.Body.Close()
	fmt.Println(resp.Status)

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		WriteJ(w, "", false)
		panic(err)
	}

	//BoltDB stuff
	imi := &Image{
		Created:   time.Now().Unix(),
		Filename:  fileName,
		RemoteURL: finURL,
	}
	err = imi.save()
	if err != nil {
		log.Println(err)
		WriteJ(w, "", false)
	}

	WriteJ(w, fileName, true)
}

func APInewImage(w http.ResponseWriter, r *http.Request) {
	contentLength := r.ContentLength
	var reader io.Reader
	var f io.WriteCloser
	var err error
	var filename string
	path := cfg.ImgDir
	vars := mux.Vars(r)
	formfilename := vars["filename"]
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
		Created:  time.Now().Unix(),
		Filename: filename,
	}
	err = imi.save()
	if err != nil {
		log.Println(err)
		WriteJ(w, "", false)
	}
	WriteJ(w, filename, true)
}

func Readme(w http.ResponseWriter, r *http.Request) {
	name := "README"
	p, err := loadPage(name, r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	body, err := ioutil.ReadFile("./" + name + ".md")
	if err != nil {
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
	err = renderTemplate(w, "md.tmpl", data)
	if err != nil {
		log.Println(err)
	}
	log.Println(name + " Page rendered!")
}

func Changelog(w http.ResponseWriter, r *http.Request) {
	name := "CHANGELOG"
	p, err := loadPage(name, r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	body, err := ioutil.ReadFile("./" + name + ".md")
	if err != nil {
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
	err = renderTemplate(w, "md.tmpl", data)
	if err != nil {
		log.Println(err)
	}
	log.Println(name + " Page rendered!")
}
