package main

//Auth functions

import (
	"github.com/gorilla/securecookie"
	//"github.com/gorilla/mux"
	"net/http"
	"log"
	"html/template"
	"time"
)
var cookieHandler = securecookie.New(
    securecookie.GenerateRandomKey(64),
    securecookie.GenerateRandomKey(32))

func SetSession(username string, w http.ResponseWriter) {
	defer timeTrack(time.Now(), "SetSession")
    value := map[string]string{
        "user": username,
    }
    if encoded, err := cookieHandler.Encode("session", value); err == nil {
        cookie := &http.Cookie{
            Name:  "session",
            Value: encoded,
            Path:  "/",
            HttpOnly: true,
        }
        http.SetCookie(w, cookie)
    }
}

func ClearSession(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{
		Name:   "session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
}

func GetUsername(r *http.Request) (username string) {
	defer timeTrack(time.Now(), "GetUsername")
	if cookie, err := r.Cookie("session"); err == nil {
		cookieValue := make(map[string]string)
		if err = cookieHandler.Decode("session", cookie.Value, &cookieValue); err == nil {
			username = cookieValue["user"]
			//log.Println(cookieValue)
		}
	} else {
		username = ""
	}
	//log.Println("GetUsername: "+username)
	return username
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	username := template.HTMLEscapeString(r.FormValue("username"))
	password := template.HTMLEscapeString(r.FormValue("password"))
	log.Println("Referrer: "+ r.Referer())
	//log.Println(r.FormValue("username"))
	//log.Println(r.FormValue("password"))
	if username == cfg.Username && password == cfg.Password {
		SetSession(username, w)
		log.Println(username + " successfully logged in.")
		w.Write([]byte("success"))
	} else {
		log.Println("LOGINHANDLER ERROR:")
		w.Write([]byte("fail"))
	}
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	ClearSession(w, r)
	log.Println("Logout")
	p, err := loadPage("Logged out", r)
	data := struct {
		Page *Page
	    Title string
	} {
		p,
		"Logged out",
	}
	err = renderTemplate(w, "login.tmpl", data)
	if err != nil {
	    log.Println(err)
	    return
	}
}

func Auth(next http.HandlerFunc) http.HandlerFunc {
    handler := func(w http.ResponseWriter, r *http.Request) {
		username := GetUsername(r)
		if username == "" {
			log.Println("AuthMiddleware mitigating: "+ r.Host + r.URL.String())
			//w.Write([]byte("OMG"))
			http.Redirect(w, r, "http://"+r.Host+"/login", 302)
			return
		} else {
		    log.Println(username + " is visiting " + r.Referer()) 	
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(handler)
}
