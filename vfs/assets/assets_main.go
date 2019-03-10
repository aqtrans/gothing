// +build !dev

package assets

import (
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/shurcooL/httpfs/vfsutil"
)

func Svg(iconName string) template.HTML {
	// MAJOR TODO:
	// Check for file existence before trying to read the file; if non-existent return ""
	iconFile, err := vfsutil.ReadFile(Assets, "icons/"+iconName+".svg")
	if err != nil {
		log.Println("Error loading assets/icons/", iconName, err)
		return template.HTML("")
	}
	return template.HTML(`<div class="svg-icon">` + string(iconFile) + `</div>`)
}

func SvgByte(iconName string) []byte {
	// MAJOR TODO:
	// Check for file existence before trying to read the file; if non-existent return ""
	iconFile, err := vfsutil.ReadFile(Assets, "icons/"+iconName+".svg")
	if err != nil {
		log.Println("Error loading assets/icons/", iconName, err)
		return []byte("")
	}
	return []byte(`<div class="svg-icon">` + string(iconFile) + `</div>`)
}

func serve(name string, w http.ResponseWriter, r *http.Request) {
	file, err := Assets.Open(name)
	if err != nil {
		log.Println("Error opening", name)
		w.Write([]byte(""))
		return
	}
	http.ServeContent(w, r, name, time.Now(), file)
	return
}

func Robots(w http.ResponseWriter, r *http.Request) {
	serve("robots.txt", w, r)
	return
}

func FaviconICO(w http.ResponseWriter, r *http.Request) {
	serve("favicon.ico", w, r)
	return
}

func FaviconPNG(w http.ResponseWriter, r *http.Request) {
	serve("favicon.png", w, r)
	return
}
