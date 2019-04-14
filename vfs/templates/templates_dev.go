// +build dev

package templates

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"git.jba.io/go/httputils"
)

var Templates http.FileSystem = http.Dir("templates")

func imgExt(s string) string {
	ext := filepath.Ext(s)
	if ext != "" {
		ext = strings.TrimLeft(ext, ".")
	}
	return ext
}

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

	funcMap := template.FuncMap{"prettyDate": httputils.PrettyDate, "safeHTML": httputils.SafeHTML, "imgClass": httputils.ImgClass, "imgExt": imgExt}

	for _, layout := range layouts {
		files := append(includes, layout)
		//DEBUG TEMPLATE LOADING
		//httputils.Debugln(files)
		templates[filepath.Base(layout)] = template.Must(template.New("templates").Funcs(funcMap).ParseFiles(files...))
	}
	return templates
}
