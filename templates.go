package godmin

import (
	"fmt"
	"html/template"
	"log"
	"strings"

	"github.com/GeertJohan/go.rice"
)

func ParseTemplates(t *template.Template) {
	fmt.Println("Parsing admin templates")
	loadTemplates(t, "index.html",
		"list.html", "change.html", "bootstrap.html",
		"navbar.html", "paginator.html")
}

func loadTemplates(templates *template.Template, list ...string) {
	templateBox, err := rice.FindBox("templates")
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
