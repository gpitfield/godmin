// Package godmin provides an admin interface for mongoDB-backed
// model to register with the godmin
// provides its own templates which are namespaced under admin/
package godmin

import (
	// "errors"
	// "html/template"
	// "encoding/json"
	// "fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type AdminAction struct {
	Identifier  string
	DisplayName string
	Action      func(values *url.Values) (err error)
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
	Count() (count int, err error)
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
	ListActions  map[string]*AdminAction
	PKStringer
	Accessor
}

func NewModelAdmin(modelName string, pkFieldName string, listFields []string,
	changeFields []string, fieldWidgets map[string]string,
	pkStringer PKStringer, accessor Accessor) (ma ModelAdmin) {
	ma = ModelAdmin{
		modelName,
		pkFieldName,
		listFields,
		changeFields,
		fieldWidgets,
		make(map[string]*AdminAction),
		pkStringer,
		accessor,
	}
	return
}

func (m *ModelAdmin) AddListAction(action *AdminAction) {
	m.ListActions[action.Identifier] = action
}

var modelAdmins = make(map[string]ModelAdmin)
var brand = "Golang Admin"
var pageSize = 100
var showPageCount = 8

// set the Brand name to show
func SetBrand(b string) {
	brand = b
}

// set the list page size
func SetPageSize(p int) {
	pageSize = p
}

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
	r.Handle("GET", "/", index)
	r.Handle("GET", "/:model/", list)
	r.Handle("POST", "/:model/", listUpdate)
	r.Handle("GET", "/:model/:pk", change)
}

func index(c *gin.Context) {
	obj := gin.H{"brand": brand, "admins": modelAdmins}
	c.HTML(200, "admin/index.html", obj)
}

// list of model instances, with dropdown actions and checkboxes
func list(c *gin.Context) {
	modelAdmin, exists := modelAdmins[strings.ToLower(c.Param("model"))]
	if !exists {
		c.String(http.StatusNotFound, "Not found.")
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "0"))
	count, _ := modelAdmin.Accessor.Count()
	totalPages := count / pageSize
	if remainder := math.Remainder(float64(count), float64(pageSize)); remainder > 0.0 {
		totalPages += 1
	}
	numPages := int(math.Min(float64(showPageCount), float64(totalPages)))
	pages := make([]int, numPages)
	startPage := int(math.Max(0, float64(page-(numPages/2))))
	endPage := int(math.Min(float64(totalPages-1), float64(startPage+numPages-1)))
	startPage = int(math.Max(0, float64(endPage-(numPages-1))))
	for i := 0; i < numPages; i++ {
		pages[i] = startPage + i
	}
	results, err := modelAdmin.Accessor.List(pageSize, page)
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
	obj := gin.H{"brand": brand, "admins": modelAdmins,
		"modelAdmin": modelAdmin, "results": mapResults, "pks": pks,
		"page": page, "pages": pages, "lastPage": totalPages - 1}
	c.HTML(200, "admin/list.html", obj)
}

func listUpdate(c *gin.Context) {
	modelAdmin, exists := modelAdmins[strings.ToLower(c.Param("model"))]
	if !exists {
		c.String(http.StatusNotFound, "Not found.")
		return
	}
	err := c.Request.ParseForm()
	if err != nil {
		log.Fatal(err)
	}
	action := c.PostForm("action")
	if listAction, exists := modelAdmin.ListActions[action]; exists {
		form := c.Request.Form
		listAction.Action(&form)
	}
	list(c)
}

// change form for model, with actions as buttons
func change(c *gin.Context) {
	modelAdmin, exists := modelAdmins[strings.ToLower(c.Param("model"))]
	if !exists {
		c.String(http.StatusNotFound, "Not found.")
		return
	}
	pk := c.Param("pk")
	if pk == "new" {
		create(c)
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
	obj := gin.H{"brand": brand, "admins": modelAdmins,
		"modelAdmin": modelAdmin, "values": ValuesMapper(modelAdmin.ChangeFields, result)}
	c.HTML(200, "admin/change.html", obj)
}

// create form for model, with actions as buttons
func create(c *gin.Context) {
	modelAdmin, exists := modelAdmins[strings.ToLower(c.Param("model"))]
	if !exists {
		c.String(http.StatusNotFound, "Not found.")
		return
	}

	results := []string{"hi", "there"}
	obj := gin.H{"brand": brand, "admins": modelAdmins,
		"currentAdmin": modelAdmin, "results": results}
	c.HTML(200, "admin/change.html", obj)
}
