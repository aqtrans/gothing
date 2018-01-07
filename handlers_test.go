package main

import (
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/boltdb/bolt"
	"jba.io/go/auth"
)

var (
	server    *httptest.Server
	reader    io.Reader //Ignore this for now
	serverUrl string
	//db, _     = bolt.Open("./data/bolt.db", 0600, nil)
	//m         *mux.Router
	//req       *http.Request
	//rr        *httptest.ResponseRecorder
)

// tempfile returns a temporary file path.
func tempfile() string {
	f, err := ioutil.TempFile("", "bolt-")
	if err != nil {
		panic(err)
	}
	if err := f.Close(); err != nil {
		panic(err)
	}
	if err := os.Remove(f.Name()); err != nil {
		panic(err)
	}
	return f.Name()
}

func TestAuthInit(t *testing.T) {
	var err error
	tmpdb := tempfile()
	defer os.Remove(tmpdb)
	authState, err := auth.NewAuthState(tmpdb)
	if err != nil {
		t.Fatal(err)
	}
	_, err = authState.Userlist()
	if err != nil {
		t.Fatal(err)
	}
}

func TestRiceInit(t *testing.T) {
	env := &thingEnv{
		templates: make(map[string]*template.Template),
	}
	err := tmplInit(env)
	if err != nil {
		t.Fatal(err)
	}
}

func TestIndexHandler(t *testing.T) {
	var aThingDB *bolt.DB
	tmpdb := tempfile()
	defer os.Remove(tmpdb)
	tmpdb2 := tempfile()
	defer os.Remove(tmpdb2)
	anAuthState, err := auth.NewAuthState(tmpdb)
	if err != nil {
		t.Fatal(err)
	}
	env := &thingEnv{
		Bolt:      &thingDB{aThingDB, tmpdb2},
		templates: make(map[string]*template.Template),
		authState: anAuthState,
	}
	err = tmplInit(env)
	if err != nil {
		t.Fatal(err)
	}
	env.dbInit()

	//db := getDB(tmpdb2)
	//defer db.Close()

	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(env.indexHandler)

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	/*
		    ctx := context.Background()
			ctx = context.WithValue(ctx, auth.UserKey, &auth.User{
				Username: "admin",
				IsAdmin: true,
			})
			params := make(map[string]string)
			params["name"] = randPage
			ctx = context.WithValue(ctx, httptreemux.ParamsContextKey, params)
			rctx := req.WithContext(ctx)
	*/

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)
	//t.Log(rr.Body.String())
	//t.Log(randPage)
	//t.Log(rr.Code)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	/*
	   // Check the response body is what we expect.
	   expected := `{"alive": true}`
	   if rr.Body.String() != expected {
	       t.Errorf("handler returned unexpected body: got %v want %v",
	           rr.Body.String(), expected)
	   }
	*/
}

func TestHelpHandler(t *testing.T) {
	var aThingDB *bolt.DB
	tmpdb := tempfile()
	defer os.Remove(tmpdb)
	tmpdb2 := tempfile()
	defer os.Remove(tmpdb2)
	anAuthState, err := auth.NewAuthState(tmpdb)
	if err != nil {
		t.Fatal(err)
	}
	env := &thingEnv{
		Bolt:      &thingDB{aThingDB, tmpdb2},
		templates: make(map[string]*template.Template),
		authState: anAuthState,
	}
	err = tmplInit(env)
	if err != nil {
		t.Fatal(err)
	}
	env.dbInit()
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/help", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(env.helpHandler)

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)
	//t.Log(rr.Body.String())
	//t.Log(randPage)
	//t.Log(rr.Code)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

}

func TestLoginPageHandler(t *testing.T) {
	var aThingDB *bolt.DB
	tmpdb := tempfile()
	defer os.Remove(tmpdb)
	tmpdb2 := tempfile()
	defer os.Remove(tmpdb2)
	anAuthState, err := auth.NewAuthState(tmpdb)
	if err != nil {
		t.Fatal(err)
	}
	env := &thingEnv{
		Bolt:      &thingDB{aThingDB, tmpdb2},
		templates: make(map[string]*template.Template),
		authState: anAuthState,
	}
	err = tmplInit(env)
	if err != nil {
		t.Fatal(err)
	}
	env.dbInit()
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/login", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(env.loginPageHandler)

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)
	//t.Log(rr.Body.String())
	//t.Log(randPage)
	//t.Log(rr.Code)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

}

func TestLookingGlassPageHandler(t *testing.T) {
	var aThingDB *bolt.DB
	tmpdb := tempfile()
	defer os.Remove(tmpdb)
	tmpdb2 := tempfile()
	defer os.Remove(tmpdb2)
	anAuthState, err := auth.NewAuthState(tmpdb)
	if err != nil {
		t.Fatal(err)
	}
	env := &thingEnv{
		Bolt:      &thingDB{aThingDB, tmpdb2},
		templates: make(map[string]*template.Template),
		authState: anAuthState,
	}
	err = tmplInit(env)
	if err != nil {
		t.Fatal(err)
	}
	env.dbInit()
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/lg", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(env.lgHandler)

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)
	//t.Log(rr.Body.String())
	//t.Log(randPage)
	//t.Log(rr.Code)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

}

func TestPastePageHandler(t *testing.T) {
	var aThingDB *bolt.DB
	tmpdb := tempfile()
	defer os.Remove(tmpdb)
	tmpdb2 := tempfile()
	defer os.Remove(tmpdb2)
	anAuthState, err := auth.NewAuthState(tmpdb)
	if err != nil {
		t.Fatal(err)
	}
	env := &thingEnv{
		Bolt:      &thingDB{aThingDB, tmpdb2},
		templates: make(map[string]*template.Template),
		authState: anAuthState,
	}
	err = tmplInit(env)
	if err != nil {
		t.Fatal(err)
	}
	env.dbInit()
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/p", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(env.pastePageHandler)

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)
	//t.Log(rr.Body.String())
	//t.Log(randPage)
	//t.Log(rr.Code)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

}

func TestFileUpPageHandler(t *testing.T) {
	var aThingDB *bolt.DB
	tmpdb := tempfile()
	defer os.Remove(tmpdb)
	tmpdb2 := tempfile()
	defer os.Remove(tmpdb2)
	anAuthState, err := auth.NewAuthState(tmpdb)
	if err != nil {
		t.Fatal(err)
	}
	env := &thingEnv{
		Bolt:      &thingDB{aThingDB, tmpdb2},
		templates: make(map[string]*template.Template),
		authState: anAuthState,
	}
	err = tmplInit(env)
	if err != nil {
		t.Fatal(err)
	}
	env.dbInit()
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/up", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(env.uploadPageHandler)

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)
	//t.Log(rr.Body.String())
	//t.Log(randPage)
	//t.Log(rr.Code)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

}

func TestImageUpPageHandler(t *testing.T) {
	var aThingDB *bolt.DB
	tmpdb := tempfile()
	defer os.Remove(tmpdb)
	tmpdb2 := tempfile()
	defer os.Remove(tmpdb2)
	anAuthState, err := auth.NewAuthState(tmpdb)
	if err != nil {
		t.Fatal(err)
	}
	env := &thingEnv{
		Bolt:      &thingDB{aThingDB, tmpdb2},
		templates: make(map[string]*template.Template),
		authState: anAuthState,
	}
	err = tmplInit(env)
	if err != nil {
		t.Fatal(err)
	}
	env.dbInit()
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/iup", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(env.uploadImagePageHandler)

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)
	//t.Log(rr.Body.String())
	//t.Log(randPage)
	//t.Log(rr.Code)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

}

func TestImageGalleryPageHandler(t *testing.T) {
	var aThingDB *bolt.DB
	tmpdb := tempfile()
	defer os.Remove(tmpdb)
	tmpdb2 := tempfile()
	defer os.Remove(tmpdb2)
	anAuthState, err := auth.NewAuthState(tmpdb)
	if err != nil {
		t.Fatal(err)
	}
	env := &thingEnv{
		Bolt:      &thingDB{aThingDB, tmpdb2},
		templates: make(map[string]*template.Template),
		authState: anAuthState,
	}
	err = tmplInit(env)
	if err != nil {
		t.Fatal(err)
	}
	env.dbInit()
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/i", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(env.galleryHandler)

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)
	//t.Log(rr.Body.String())
	//t.Log(randPage)
	//t.Log(rr.Code)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

}

func BenchmarkIndex(b *testing.B) {

	var aThingDB *bolt.DB
	tmpdb := tempfile()
	defer os.Remove(tmpdb)
	tmpdb2 := tempfile()
	defer os.Remove(tmpdb2)
	anAuthState, err := auth.NewAuthState(tmpdb)
	if err != nil {
		b.Fatal(err)
	}
	env := &thingEnv{
		Bolt:      &thingDB{aThingDB, tmpdb2},
		templates: make(map[string]*template.Template),
		authState: anAuthState,
	}
	err = tmplInit(env)
	if err != nil {
		b.Fatal(err)
	}
	env.dbInit()

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		b.Fatal(err)
	}
	req.Host = "squanch.gg"

	handler := http.HandlerFunc(env.indexHandler)

	rr := httptest.NewRecorder()

	for n := 0; n < b.N; n++ {
		handler.ServeHTTP(rr, req)
	}
}
