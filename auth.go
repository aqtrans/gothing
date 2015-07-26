package main

//Auth functions

import (
	"github.com/gorilla/securecookie"
	"github.com/mavricknz/ldap"
	//"github.com/gorilla/mux"
	"html/template"
	"log"
	"fmt"
	"net/http"
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
			Name:     "session",
			Value:    encoded,
			Path:     "/",
			HttpOnly: true,
		}
		http.SetCookie(w, cookie)
	}
}

func ClearSession(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
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
	log.Println("Referrer: " + r.Referer())
	//log.Println(r.FormValue("username"))
	//log.Println(r.FormValue("password"))
	
	// LDAP
	//if username == cfg.Username && password == cfg.Password {
	if ldapAuth(username, password) {	
		SetSession(username, w)
		log.Println(username + " successfully logged in.")
		WriteJ(w, "", true)
	} else {
		WriteJ(w, "", false)
	}
}

func ldapAuth(un, pw string) bool {
	//Build DN: uid=admin,ou=People,dc=example,dc=com
	dn := cfg.LDAPun+"="+un+","+cfg.LDAPdn
	l := ldap.NewLDAPConnection(cfg.LDAPurl, cfg.LDAPport)
	err := l.Connect()
	if err != nil {
		fmt.Print(dn)
		fmt.Printf("LDAP connectiong error: %v", err)
		return false
	}
	defer l.Close()
	err = l.Bind(dn, pw)
	if err != nil {
		fmt.Print(dn)
		fmt.Printf("error: %v", err)
		return false
	}
	fmt.Print("Authenticated")
	return true			
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	ClearSession(w, r)
	log.Println("Logout")
	http.Redirect(w, r, r.Referer(), 302)
}

func Auth(next http.HandlerFunc) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		username := GetUsername(r)
		if username == "" {
			log.Println("AuthMiddleware mitigating: " + r.Host + r.URL.String())
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
