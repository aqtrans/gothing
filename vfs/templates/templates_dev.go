// +build dev

package templates

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"git.jba.io/go/httputils"
	"git.jba.io/go/wiki/vfs/assets"
)

var Templates http.FileSystem = http.Dir("templates")

// TmplInit() initializes a map of templates, in this case using local FS
func TmplInit() map[string]*template.Template {
	templates := make(map[string]*template.Template)

	templatesDir := "./templates/"
	layouts, err := filepath.Glob(templatesDir + "layouts/*.tmpl")
	if err != nil {
		log.Fatalln(err)
	}
	includes, err := filepath.Glob(templatesDir + "includes/*.tmpl")
	if err != nil {
		log.Fatalln(err)
	}

	funcMap := template.FuncMap{"svg": assets.Svg, "typeIcon": typeIcon, "prettyDate": httputils.PrettyDate, "safeHTML": httputils.SafeHTML, "imgClass": httputils.ImgClass, "isLoggedIn": isLoggedIn, "jsTags": jsTags}

	for _, layout := range layouts {
		files := append(includes, layout)
		//DEBUG TEMPLATE LOADING
		//httputils.Debugln(files)
		templates[filepath.Base(layout)] = template.Must(template.New("templates").Funcs(funcMap).ParseFiles(files...))
	}
	return templates
}
