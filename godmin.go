// Package godmin provides an admin interface for arbitrary structs
// to register with a gin-based html admin, similar to Django's admin interface.
package godmin

import (
	// "errors"
	// "encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	templ "github.com/gpitfield/godmin/template"
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

func lowerLettersOnly(r rune) rune {
	switch {
	case r == '-' || r >= 'a' && r <= 'z':
		return r
	default:
		return -1
	}
}

// NewAdminAction returns a pointer to a new AdminAction instance.
func NewAdminAction(displayName string, confirm bool, confirmTitle string,
	confirmMsg string, action func(values *url.Values) (err error)) *AdminAction {

	identifier := strings.Replace(strings.ToLower(displayName), " ", "-", -1)
	identifier = strings.Map(lowerLettersOnly, identifier)
	return &AdminAction{
		Identifier:     identifier,
		DisplayName:    displayName,
		Confirm:        confirm,
		ConfirmTitle:   confirmTitle,
		ConfirmMessage: confirmMsg,
		Action:         action,
	}
}

// Authenticator manages admin rights. If no Authenticator is provided, the admin is completely open.
// This is of course strongly not recommended.
type Authenticator interface {
	// middleware handler that validates whether a user is logged in,
	// and sets the "username", "accountId" values in the context variable if so
	IsAdmin(c *gin.Context) (ok bool)
	// function that returns whether the request has the necessary privilege for the desired operation
	HasPrivilege(c *gin.Context, collection string, action string, ids []string) (ok bool)
}

// Accessor is implemented by types wishing to register their structs
// with the admin. It enables the admin to read/write administered objects.
type Accessor interface {
	// Return an single empty instance of the model
	Prototype() (result interface{})
	PrototypePtr() (result *struct{})
	// Must return a single struct of the administered type
	Get(pk string) (result interface{}, err error)
	// Must return a slice of structs of the administered type
	List(count, page int, order []Order) (results interface{}, err error)
	Count() (count int, err error)
	Upsert(pk string, values map[string][]string) (outPk string, err error)
	DeletePK(pk string) (err error)
}

// Seacher provides a list of fields (for admin users to see what's being searched)
// and a search function that returns a list of results based on the provided url.Values,
// the count per page, the page number, and any sort order.
type Searcher struct {
	Placeholder string // preferred to indicate fields that are being searched
	Search      func(count, page int, query string, order []Order) (results interface{}, totalCount int, err error)
}

// Order provides the information necessary to sort on a field
type Order struct {
	FieldName string
	Ascending bool
}

// PKStringer converts the primary key to a string if it isn't one
type PKStringer interface {
	PKString(pk interface{}) string
}

// meta information to register the model with the admin
// note that field order is controlled by the admin, which currently sorts alphabetically by field name
// which is the default result of iterating a map in a template.
// TODO: we may want to revisit this approach to ordering.
type ModelAdmin struct {
	ModelName      string // database table/collection name
	PKFieldName    string
	ListFields     map[string]bool   // optional fields to be shown in list views. True if sortable, else false
	OmitFields     map[string]bool   // optional fields to omit from change view
	ReadOnlyFields map[string]bool   // optional read-only fields for change view
	FieldNotes     map[string]string // optional note about the field
	FieldWidgets   map[string]string // optional type of widget to render with
	ListActions    map[string]*AdminAction
	PKStringer
	Accessor
	*Searcher
}

// return a new ModelAdmin from the supplied arguments
func NewModelAdmin(modelName string, pkFieldName string, listFields map[string]bool,
	omitFields map[string]bool, readOnlyFields map[string]bool, fieldNotes map[string]string,
	fieldWidgets map[string]string, pkStringer PKStringer, accessor Accessor, searcher *Searcher) (ma ModelAdmin) {
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
		searcher,
	}
	return
}

// registers an AdminAction for use in the list view
func (m *ModelAdmin) AddListAction(action *AdminAction) {
	m.ListActions[action.Identifier] = action
}

var (
	adminPath     = "/admin"
	accountIdKey  = "accountId"
	usernameKey   = "username"
	modelAdmins   = make(map[string]ModelAdmin)
	brand         = "Golang Admin"
	pageSize      = 100
	showPageCount = 8
	loginURL      string
	logoutURL     string
	authenticator Authenticator
)

//set set the Admin path
func SetAdminPath(p string) {
	adminPath = p
}

// set the Authenticator
func SetAuthenticator(a Authenticator) {
	authenticator = a
}

// set the Brand name to show
func SetBrand(b string) {
	brand = b
}

// set the list page size
func SetPageSize(p int) {
	pageSize = p
}

// set the URL where a user can log in
func SetLoginURL(url string) {
	loginURL = url
}

// set the URL where a user can log out
func SetLogoutURL(url string) {
	logoutURL = url
}

// register a ModelAdmin instance to be available in the admin
func Register(ma ModelAdmin) {
	lcModelName := strings.ToLower(ma.ModelName)
	if _, exists := modelAdmins[lcModelName]; exists {
		log.Println(ma.ModelName, "Model Admin already registered")
	}
	if ma.FieldWidgets == nil {
		ma.FieldWidgets = defaultWidgets(ma)
	}
	modelAdmins[lcModelName] = ma
}

func defaultDot(c *gin.Context) map[string]interface{} {
	dot := gin.H{"brand": brand, "adminPath": adminPath, "loginURL": loginURL, "logoutURL": logoutURL,
		"admins": modelAdmins}
	if accountId, exists := c.Get(accountIdKey); exists {
		dot["accountId"] = accountId
	}
	if username, exists := c.Get(usernameKey); exists {
		dot["username"] = username
	}
	return dot
}

// set up the admin Routes, and add in the Authenticator middleware if present
func Routes(r *gin.RouterGroup) {
	// root level is list of admin models
	r.Handle("GET", "/", index)
	r.Handle("GET", "/:model/", list)
	r.Handle("POST", "/:model/", listUpdate)
	r.Handle("GET", "/:model/:pk", change)
	r.Handle("POST", "/:model/:pk", changeUpdate)
}

func ParseTemplates(t *template.Template) {
	fmt.Println("Parsing admin templates")
	templ.LoadTemplates(t, "index.html",
		"list.html", "change.html", "bootstrap.html",
		"navbar.html", "paginator.html", "confirmModal.html",
		"tableWidgets.html", "formWidgets.html", "error.html",
		"footer.html")
}

// Check for permission issues via the status code set by the Authenticator
func hasPermissions(c *gin.Context, collection string, action string, ids []string) (ok bool) {
	dot := defaultDot(c)
	if !authenticator.IsAdmin(c) {
		dot["error"] = "Please log in with an admin account."
		c.HTML(200, "admin/error.html", dot)
		return false
	}
	if !authenticator.HasPrivilege(c, collection, action, ids) {
		dot["error"] = "You don't have the necessary permissions to do that."
		c.HTML(200, "admin/error.html", dot)
		return false
	}
	return true
}

// Admin home page
func index(c *gin.Context) {
	if !hasPermissions(c, "", "read", nil) {
		return
	}
	var objectCounts = make(map[string]int)
	for model, admin := range modelAdmins {
		count, _ := admin.Accessor.Count()
		objectCounts[model] = count
	}
	dot := defaultDot(c)
	dot["counts"] = objectCounts
	c.HTML(200, "admin/index.html", dot)
}

// list of model instances, with dropdown actions and checkboxes
func list(c *gin.Context) {
	var (
		results interface{}
		err     error
		page    int
		count   int
		query   string
		sort    string
		order   []Order
		orders  = make(map[string]int)
	)
	modelAdmin, exists := modelAdmins[strings.ToLower(c.Param("model"))]
	if !exists {
		c.String(http.StatusNotFound, "Not found.")
		return
	}
	if !hasPermissions(c, modelAdmin.ModelName, "read", nil) {
		return
	}
	page, err = strconv.Atoi(c.DefaultQuery("page", "0"))
	query = c.Query("q")
	sort = c.Query("o")
	for field, _ := range modelAdmin.ListFields {
		orders[field] = 0
	}
	if sort != "" {
		field := strings.TrimPrefix(sort, "-")
		ascend := true
		if strings.HasPrefix(sort, "-") {
			ascend = false
		}
		order = []Order{Order{field, ascend}}
		if ascend {
			orders[field] = 1
		} else {
			orders[field] = -1
		}
	}

	if modelAdmin.Searcher == nil || query == "" {
		results, err = modelAdmin.Accessor.List(pageSize, page, order)
		count, _ = modelAdmin.Accessor.Count()
	} else {
		results, count, err = modelAdmin.Searcher.Search(pageSize, page, query, order)
	}
	if err != nil {
		log.Fatal("error in godmin list:", err)
	}

	totalPages := count / pageSize
	if remainder := math.Mod(float64(count), float64(pageSize)); remainder > 0.0 {
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

	resultValues := reflect.ValueOf(results)
	resultCount := resultValues.Len()
	mapResults := make([][]AdminField, resultCount, resultCount)
	pks := make([]string, resultCount, resultCount)
	for i := 0; i < resultCount; i++ {
		mapResults[i] = Marshal(resultValues.Index(i).Interface(), modelAdmin, "")
		pks[i] = modelAdmin.PKStringer.PKString(resultValues.Index(i).FieldByName(modelAdmin.PKFieldName).Interface())
	}
	dot := defaultDot(c)
	dot["modelAdmin"] = modelAdmin
	dot["results"] = mapResults
	dot["pks"] = pks
	dot["page"] = page
	dot["pages"] = pages
	dot["lastPage"] = totalPages - 1
	dot["query"] = query
	dot["orders"] = orders
	dot["sort"] = sort
	if modelAdmin.Searcher != nil {
		dot["search"] = true
		dot["searchPlaceholder"] = modelAdmin.Searcher.Placeholder
	}
	c.HTML(200, "admin/list.html", dot)
}

// handle actions to be executed on a set of objects from a model's list view
func listUpdate(c *gin.Context) {
	modelAdmin, exists := modelAdmins[strings.ToLower(c.Param("model"))]
	if !exists {
		c.String(http.StatusNotFound, "Not found.")
		return
	}
	if !hasPermissions(c, modelAdmin.ModelName, "write", nil) { // TODO: add in the IDs
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
		if hasPermissions(c, modelAdmin.ModelName, "create", nil) {
			create(c)
		}
		return
	}
	if !hasPermissions(c, modelAdmin.ModelName, "write", []string{pk}) {
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
	dot := defaultDot(c)
	dot["modelAdmin"] = modelAdmin
	dot["values"] = ValuesMapper(result)
	dot["pk"] = pk
	c.HTML(200, "admin/change.html", dot)
}

// upsert an object from HTML form values
func saveFromForm(c *gin.Context) {
	log.Println("hitting SaveFromForm")
	modelAdmin, exists := modelAdmins[strings.ToLower(c.Param("model"))]
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
	log.Println("form", form)
	objectMap := Unmarshal(form, &modelAdmin)
	log.Println(objectMap)
	// proto := modelAdmin.Accessor.Prototype()
	if len(objectMap) > 0 {
		_, err = modelAdmin.Accessor.Upsert(pk, objectMap)
	}
	if err != nil {
		log.Println(err)
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
	log.Println("hitting changeUpdate")
	action := c.DefaultPostForm("action", "save")
	delete(c.Request.Form, "action") // don't keep this as part of the object
	modelAdmin, exists := modelAdmins[strings.ToLower(c.Param("model"))]
	if !exists {
		c.String(http.StatusNotFound, "Not found.")
		return
	}
	if !hasPermissions(c, modelAdmin.ModelName, "write", nil) { // TODO: add in the ID(s)
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
		modelAdmin.Accessor.DeletePK(c.Param("pk"))
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
	dot := defaultDot(c)
	dot["modelAdmin"] = modelAdmin
	dot["pk"] = "add"
	dot["values"] = ValuesMapper(result)
	c.HTML(200, "admin/change.html", dot)
}
