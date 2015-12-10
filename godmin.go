// Package godmin provides an admin interface for arbitrary
// structs to register with a gin-based html admin, similar to
// Django's admin interface.
package godmin

import (
	// "errors"
	// "html/template"
	// "encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// AdminAction defines actions that can be taken on objects in the admin
type AdminAction struct {
	Identifier     string
	DisplayName    string
	Confirm        bool
	ConfirmTitle   string
	ConfirmMessage string
	Action         func(values *url.Values) (err error)
}

// The Accessor interface is implemented by types wishing to register their structs
// with the admin. It enables the admin to read/write administered objects.
type Accessor interface {
	// Return an single empty instance of the model
	Prototype() (result interface{})
	PrototypePtr() (result *struct{})
	// Must return a single struct of the administered type
	Get(pk string) (result interface{}, err error)
	// Must return a slice of structs of the administered type
	List(count, page int) (results interface{}, err error)
	Count() (count int, err error)
	Upsert(pk string, item interface{}) (outPk string, err error)
	Delete(pk string) (err error)
}

// type Structer interface {
// 	ProtoStruct() (result struct{})
// }

// convert the primary key to a string if it isn't one
type PKStringer interface {
	PKString(pk interface{}) string
}

// meta information to register the model with the admin
type ModelAdmin struct {
	ModelName      string // database table/collection name
	PKFieldName    string
	ListFields     map[string]bool   // optional fields to be shown in list views.
	OmitFields     map[string]bool   // optional fields to omit from change view
	ReadOnlyFields map[string]bool   // optional read-only fields for change view
	FieldNotes     map[string]string // optional note about the field
	FieldWidgets   map[string]string // optional type of widget to render with
	ListActions    map[string]*AdminAction
	PKStringer
	Accessor
}

// return a new ModelAdmin from the supplied arguments
func NewModelAdmin(modelName string, pkFieldName string, listFields map[string]bool,
	omitFields map[string]bool, readOnlyFields map[string]bool, fieldNotes map[string]string,
	fieldWidgets map[string]string, pkStringer PKStringer, accessor Accessor) (ma ModelAdmin) {
	ma = ModelAdmin{
		modelName,
		pkFieldName,
		listFields,
		omitFields,
		readOnlyFields,
		fieldNotes,
		fieldWidgets,
		make(map[string]*AdminAction),
		pkStringer,
		accessor,
	}
	return
}

// registers an AdminAction for use in the list view
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
		log.Println(ma.ModelName, "Model Admin already registered")
	}
	log.Println("Registering", ma.ModelName, "admin")
	if ma.FieldWidgets == nil {
		ma.FieldWidgets = defaultWidgets(ma)
	}
	modelAdmins[lcModelName] = ma
}

func Routes(r *gin.RouterGroup) {
	// root level is list of admin models
	r.Handle("GET", "/", index)
	r.Handle("GET", "/:model/", list)
	r.Handle("POST", "/:model/", listUpdate)
	r.Handle("GET", "/:model/:pk", change)
	r.Handle("POST", "/:model/:pk", changeUpdate)
}

// Admin home page
func index(c *gin.Context) {
	var objectCounts = make(map[string]int)
	for model, admin := range modelAdmins {
		count, _ := admin.Accessor.Count()
		objectCounts[model] = count
	}
	obj := gin.H{"brand": brand, "admins": modelAdmins, "counts": objectCounts}
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
	mapResults := make([][]AdminField, resultCount, resultCount)
	pks := make([]string, resultCount, resultCount)
	for i := 0; i < resultCount; i++ {
		mapResults[i] = Marshal(resultValues.Index(i).Interface(), modelAdmin, "")
		pks[i] = modelAdmin.PKStringer.PKString(resultValues.Index(i).FieldByName(modelAdmin.PKFieldName).Interface())
	}
	obj := gin.H{"brand": brand, "admins": modelAdmins,
		"modelAdmin": modelAdmin, "results": mapResults, "pks": pks,
		"page": page, "pages": pages, "lastPage": totalPages - 1}
	c.HTML(200, "admin/list.html", obj)
}

// handle actions to be executed on a set of objects from a model's list view
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
	if pk == "add" {
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
		"modelAdmin": modelAdmin, "values": ValuesMapper(result), "pk": pk}
	c.HTML(200, "admin/change.html", obj)
}

// upsert an object from HTML form values
func saveFromForm(c *gin.Context) {
	_, exists := modelAdmins[strings.ToLower(c.Param("model"))]
	if !exists {
		c.String(http.StatusNotFound, "Not found.")
		return
	}
	pk := c.Param("pk")
	if pk == "add" {
		pk = ""
	}
	err := c.Request.ParseForm()
	if err != nil {
		log.Fatal(err)
	}
	form := c.Request.Form
	fmt.Println("form", form.Get("Languages"))
	// proto := modelAdmin.Accessor.Prototype()
	// _, err = modelAdmin.Accessor.Upsert(pk, proto)
	if err != nil {
		if err.Error() == "Not Found" {
			c.String(http.StatusNotFound, "Not found.")
			return
		}
		if err.Error() == "Invalid ID" {
			c.String(http.StatusNotFound, "Invalid ID.")
			return
		}
		c.String(http.StatusNotAcceptable, err.Error())
		return
	}
}

// update an object from its change form
func changeUpdate(c *gin.Context) {
	action := c.DefaultPostForm("action", "save")
	modelAdmin, exists := modelAdmins[strings.ToLower(c.Param("model"))]
	if !exists {
		c.String(http.StatusNotFound, "Not found.")
		return
	}
	switch action {
	case "save":
		saveFromForm(c)
		c.Request.Method = "GET"
		c.Redirect(http.StatusFound, fmt.Sprintf("../%v", strings.ToLower(c.Param("model"))))
	case "save-continue":
		saveFromForm(c)
		change(c)
	case "delete":
		modelAdmin.Accessor.Delete(c.Param("pk"))
		c.Request.Method = "GET"
		c.Redirect(http.StatusFound, fmt.Sprintf("../%v", strings.ToLower(c.Param("model"))))
	}
}

// create form for model, with actions as buttons
func create(c *gin.Context) {
	modelAdmin, exists := modelAdmins[strings.ToLower(c.Param("model"))]
	if !exists {
		c.String(http.StatusNotFound, "Not found.")
		return
	}
	result := modelAdmin.Accessor.Prototype()
	obj := gin.H{"brand": brand, "admins": modelAdmins,
		"modelAdmin": modelAdmin, "pk": "add", "values": ValuesMapper(result)}
	c.HTML(200, "admin/change.html", obj)
}
