package template

import (
	"html/template"
	"log"
	"strings"

	"github.com/GeertJohan/go.rice"
)

func LoadTemplates(templates *template.Template, list ...string) {
	templateBox, err := rice.FindBox(".")
	if err != nil {
		log.Fatal(err)
	}
	funcMap := template.FuncMap{
		"add": func(x, y int) int { return x + y },
	}
	for _, x := range list {
		templateString, err := templateBox.String(x)
		if err != nil {
			log.Fatal(err)
		}
		_, err = templates.New(strings.Join([]string{"admin/", x}, "")).Funcs(funcMap).Parse(templateString)
		if err != nil {
			log.Fatal(err)
		}
	}
}
