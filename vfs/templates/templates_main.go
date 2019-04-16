// +build !dev

package templates

import (
	"html/template"
	"log"
	"path/filepath"
	"strings"

	"git.jba.io/go/httputils"
	"github.com/shurcooL/httpfs/html/vfstemplate"
	"github.com/shurcooL/httpfs/path/vfspath"
)

func imgExt(s string) string {
	ext := filepath.Ext(s)
	if ext != "" {
		ext = strings.TrimLeft(ext, ".")
	}
	return ext
}

func TmplInit() map[string]*template.Template {
	templates := make(map[string]*template.Template)

	layouts, err := vfspath.Glob(Templates, "layouts/*.tmpl")
	if err != nil {
		log.Fatalln(err)
	}
	includes, err := vfspath.Glob(Templates, "includes/*.tmpl")
	if err != nil {
		log.Fatalln(err)
	}

	funcMap := template.FuncMap{"prettyDate": httputils.PrettyDate, "safeHTML": httputils.SafeHTML, "imgClass": httputils.ImgClass, "imgExt": imgExt}

	for _, layout := range layouts {
		files := append(includes, layout)
		//DEBUG TEMPLATE LOADING
		//httputils.Debugln(files)
		tmpl, err := vfstemplate.ParseFiles(Templates, template.New("templates").Funcs(funcMap), files...)
		if err != nil {
			log.Fatalln(err)
		}
		templates[filepath.Base(layout)] = tmpl
	}
	return templates
}
