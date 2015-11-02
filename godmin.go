// Package godmin provides an admin interface for mongoDB-backed
// model to register with the godmin
// provides its own templates which are namespaced under admin/
package godmin

import (
	// "errors"
	// "fmt"
	// "html/template"
	// "encoding/json"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type AdminAction interface {
	// tbd how this should actually be set up
}

type AdminField struct {
	FieldName  string
	FieldType  string
	TextValue  string
	FloatValue float64
	IntValue   int
	ReadOnly   bool
}

// The Accessor interface is implemented by types wishing to register their structs
// with the admin. It enables the admin to read/write administered objects.
type Accessor interface {
	// Must return a single struct of the administered type
	Get(pk string) (result interface{}, err error)
	// Must return a slice of structs of the administered type
	List(count, page int) (results interface{}, err error)
	// Upsert(items []interface{}) error
}

// convert the primary key to a string if it isn't one
type PKStringer interface {
	PKString(pk interface{}) string
}

// meta information to register the model with the admin
type ModelAdmin struct {
	ModelName    string // database table/collection name
	PKFieldName  string
	ListFields   []string          // optional fields to be shown in list views.
	ChangeFields []string          // optional fields to be shown in change view
	FieldWidgets map[string]string // optional type of widget to render with
	PKStringer
	Accessor
}

var modelAdmins = make(map[string]ModelAdmin)

// register a ModelAdmin instance to be available in the admin
func Register(ma ModelAdmin) {
	lcModelName := strings.ToLower(ma.ModelName)
	if _, exists := modelAdmins[lcModelName]; exists {
		log.Fatalln(ma.ModelName, "Model Admin already registered")
	}
	log.Println("Registering", ma.ModelName, "admin")
	modelAdmins[lcModelName] = ma
}

func Routes(r *gin.RouterGroup) {
	// root level is list of admin models
	r.Handle("GET", "/", indexRoute)
	r.Handle("GET", "/:model/", listRoute)
	r.Handle("GET", "/:model/:pk", changeRoute)
}

func indexRoute(c *gin.Context) {
	obj := gin.H{"title": "GitLance Admin", "admins": modelAdmins}
	c.HTML(200, "admin/index.html", obj)
}

// list of model instances, with dropdown actions and checkboxes
func listRoute(c *gin.Context) {
	modelAdmin, exists := modelAdmins[strings.ToLower(c.Param("model"))]
	if !exists {
		c.String(http.StatusNotFound, "Not found.")
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "0"))
	count, err := strconv.Atoi(c.DefaultQuery("count", "100")) // TODO: don't hard code default
	results, err := modelAdmin.Accessor.List(count, page)
	if err != nil {
		log.Fatal(err)
	}

	resultValues := reflect.ValueOf(results)
	resultCount := resultValues.Len()
	mapResults := make([]map[string]string, resultCount, resultCount)
	pks := make([]string, resultCount, resultCount)
	for i := 0; i < resultCount; i++ {
		mapResults[i] = ValuesMapper(modelAdmin.ListFields, resultValues.Index(i).Interface())
		pks[i] = modelAdmin.PKStringer.PKString(resultValues.Index(i).FieldByName(modelAdmin.PKFieldName).Interface())
	}

	obj := gin.H{"title": "GitLance Admin", "admins": modelAdmins,
		"modelAdmin": modelAdmin, "results": mapResults, "pks": pks}
	c.HTML(200, "admin/list.html", obj)
}

// change form for model, with actions as buttons
func changeRoute(c *gin.Context) {
	modelAdmin, exists := modelAdmins[strings.ToLower(c.Param("model"))]
	if !exists {
		c.String(http.StatusNotFound, "Not found.")
		return
	}
	pk := c.Param("pk")
	if pk == "new" {
		createRoute(c)
		return
	}

	result, err := modelAdmin.Accessor.Get(pk)
	if err != nil {
		if err.Error() == "Not Found" {
			c.String(http.StatusNotFound, "Not found.")
			return
		}
		if err.Error() == "Invalid ID" {
			c.String(http.StatusNotFound, "Invalid ID.")
			return
		}
		log.Fatal(err)
	}
	obj := gin.H{"title": "GitLance Admin", "admins": modelAdmins,
		"modelAdmin": modelAdmin, "values": ValuesMapper(modelAdmin.ChangeFields, result)}
	c.HTML(200, "admin/change.html", obj)
}

// create form for model, with actions as buttons
func createRoute(c *gin.Context) {
	modelAdmin, exists := modelAdmins[strings.ToLower(c.Param("model"))]
	if !exists {
		c.String(http.StatusNotFound, "Not found.")
		return
	}

	results := []string{"hi", "there"}
	obj := gin.H{"title": "GitLance Admin", "admins": modelAdmins,
		"currentAdmin": modelAdmin, "results": results}
	c.HTML(200, "admin/change.html", obj)
}
