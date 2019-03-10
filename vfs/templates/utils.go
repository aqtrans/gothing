package templates

import (
	"html/template"
	"strings"

	"git.jba.io/go/wiki/vfs/assets"
)

func typeIcon(gitType string) template.HTML {
	var html template.HTML
	if gitType == "blob" {
		html = assets.Svg("file-text")
	}
	if gitType == "tree" {
		html = assets.Svg("folder-open")
	}
	return html
}

func isLoggedIn(s string) bool {
	if s == "" {
		return false
	}
	return true
}

func jsTags(tagS []string) string {
	var tags string
	for _, v := range tagS {
		tags = tags + ", " + v
	}
	tags = strings.TrimPrefix(tags, ", ")
	tags = strings.TrimSuffix(tags, ", ")
	return tags
}
