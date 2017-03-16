package main

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/GeertJohan/go.rice"
	"github.com/boltdb/bolt"
	"jba.io/go/auth"
	"jba.io/go/httputils"
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

type DB struct {
	*bolt.DB
}

// MustOpenDB returns a new, open DB at a temporary location.
func mustOpenDB() *DB {
	tmpdb, err := bolt.Open(tempfile(), 0666, nil)
	if err != nil {
		panic(err)
	}
	return &DB{tmpdb}
}

func (tmpdb *DB) Close() error {
	defer os.Remove(tmpdb.Path())
	return tmpdb.DB.Close()
}

func (tmpdb *DB) MustClose() {
	if err := tmpdb.Close(); err != nil {
		panic(err)
	}
}

type TestAuthDB struct {
	*auth.DB
}

// MustOpenDB returns a new, open DB at a temporary location.
func mustOpenAuthDB() *TestAuthDB {
	tmpdb, err := bolt.Open(tempfile(), 0666, nil)
	if err != nil {
		panic(err)
	}
	return &TestAuthDB{&auth.DB{tmpdb}}
}

func (tmpdb *TestAuthDB) Close() error {
	//log.Println(tmpdb.Path())
	defer os.Remove(tmpdb.Path())
	return tmpdb.DB.Close()
}

func (tmpdb *TestAuthDB) MustClose() {
	if err := tmpdb.Close(); err != nil {
		panic(err)
	}
}

func TestAuthInit(t *testing.T) {
	var err error
	authDB := mustOpenAuthDB()
	authState, err = auth.NewAuthStateWithDB(authDB.DB, tempfile(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer authDB.Close()
	_, err = authState.Userlist()
	if err != nil {
		t.Fatal(err)
	}
}

func TestRiceInit(t *testing.T) {
	httputils.AssetsBox = rice.MustFindBox("assets")
	err := riceInit()
	if err != nil {
		t.Fatal(err)
	}
}

func TestIndexHandler(t *testing.T) {
	var err error
	authDB := mustOpenAuthDB()
	authState, err = auth.NewAuthStateWithDB(authDB.DB, tempfile(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer authDB.Close()

	tmpdb := mustOpenDB()
	t.Log(tmpdb.Path())
	db = tmpdb.DB
	dbInit()
	defer tmpdb.MustClose()
	httputils.AssetsBox = rice.MustFindBox("assets")
	err = riceInit()
	if err != nil {
		t.Fatal(err)
	}
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(indexHandler)

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
	var err error
	authDB := mustOpenAuthDB()
	authState, err = auth.NewAuthStateWithDB(authDB.DB, tempfile(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer authDB.Close()

	tmpdb := mustOpenDB()
	t.Log(tmpdb.Path())
	db = tmpdb.DB
	dbInit()
	defer tmpdb.MustClose()
	httputils.AssetsBox = rice.MustFindBox("assets")
	err = riceInit()
	if err != nil {
		t.Fatal(err)
	}
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/help", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(helpHandler)

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
	var err error
	authDB := mustOpenAuthDB()
	authState, err = auth.NewAuthStateWithDB(authDB.DB, tempfile(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer authDB.Close()

	tmpdb := mustOpenDB()
	t.Log(tmpdb.Path())
	db = tmpdb.DB
	dbInit()
	defer tmpdb.MustClose()
	httputils.AssetsBox = rice.MustFindBox("assets")
	err = riceInit()
	if err != nil {
		t.Fatal(err)
	}
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/login", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(loginPageHandler)

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
	var err error
	authDB := mustOpenAuthDB()
	authState, err = auth.NewAuthStateWithDB(authDB.DB, tempfile(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer authDB.Close()

	tmpdb := mustOpenDB()
	t.Log(tmpdb.Path())
	db = tmpdb.DB
	dbInit()
	defer tmpdb.MustClose()
	httputils.AssetsBox = rice.MustFindBox("assets")
	err = riceInit()
	if err != nil {
		t.Fatal(err)
	}
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/lg", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(lgHandler)

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
	var err error
	authDB := mustOpenAuthDB()
	authState, err = auth.NewAuthStateWithDB(authDB.DB, tempfile(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer authDB.Close()

	tmpdb := mustOpenDB()
	t.Log(tmpdb.Path())
	db = tmpdb.DB
	dbInit()
	defer tmpdb.MustClose()
	httputils.AssetsBox = rice.MustFindBox("assets")
	err = riceInit()
	if err != nil {
		t.Fatal(err)
	}
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/p", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(pastePageHandler)

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
	var err error
	authDB := mustOpenAuthDB()
	authState, err = auth.NewAuthStateWithDB(authDB.DB, tempfile(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer authDB.Close()

	tmpdb := mustOpenDB()
	t.Log(tmpdb.Path())
	db = tmpdb.DB
	dbInit()
	defer tmpdb.MustClose()
	httputils.AssetsBox = rice.MustFindBox("assets")
	err = riceInit()
	if err != nil {
		t.Fatal(err)
	}
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/up", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(uploadPageHandler)

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
	var err error
	authDB := mustOpenAuthDB()
	authState, err = auth.NewAuthStateWithDB(authDB.DB, tempfile(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer authDB.Close()

	tmpdb := mustOpenDB()
	t.Log(tmpdb.Path())
	db = tmpdb.DB
	dbInit()
	defer tmpdb.MustClose()
	httputils.AssetsBox = rice.MustFindBox("assets")
	err = riceInit()
	if err != nil {
		t.Fatal(err)
	}
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/iup", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(uploadImagePageHandler)

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
	var err error
	authDB := mustOpenAuthDB()
	authState, err = auth.NewAuthStateWithDB(authDB.DB, tempfile(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer authDB.Close()

	tmpdb := mustOpenDB()
	t.Log(tmpdb.Path())
	db = tmpdb.DB
	dbInit()
	defer tmpdb.MustClose()
	httputils.AssetsBox = rice.MustFindBox("assets")
	err = riceInit()
	if err != nil {
		t.Fatal(err)
	}
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/i", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(galleryHandler)

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
