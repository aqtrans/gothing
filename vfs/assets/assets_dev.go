// +build dev

package assets

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
)

// Assets contains project assets.
var Assets http.FileSystem = http.Dir("assets")

func Svg(iconName string) template.HTML {
	// MAJOR TODO:
	// Check for file existence before trying to read the file; if non-existent return ""
	iconFile, err := ioutil.ReadFile("assets/icons/" + iconName + ".svg")
	if err != nil {
		log.Println("Error loading assets/icons/", iconName, err)
		return template.HTML("")
	}
	return template.HTML(`<div class="svg-icon">` + string(iconFile) + `</div>`)
}

func SvgByte(iconName string) []byte {
	// MAJOR TODO:
	// Check for file existence before trying to read the file; if non-existent return ""
	iconFile, err := ioutil.ReadFile("assets/icons/" + iconName + ".svg")
	if err != nil {
		log.Println("Error loading assets/icons/", iconName, err)
		return []byte("")
	}
	return []byte(`<div class="svg-icon">` + string(iconFile) + `</div>`)
}

func Robots(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./assets/robots.txt")
	return
}

func FaviconICO(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./assets/favicon.ico")
	return
}

func FaviconPNG(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./assets/favicon.png")
	return
}
