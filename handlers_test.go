package main

import (
	"io"
	"testing"
	"net/http"
	"net/http/httptest"
)

var (
	server    *httptest.Server
	reader    io.Reader //Ignore this for now
	serverUrl string
	//m         *mux.Router
	//req       *http.Request
	//rr        *httptest.ResponseRecorder
)

func TestIndexHandler(t *testing.T) {
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