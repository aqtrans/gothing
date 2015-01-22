package moes 

import ( 
	"net/http"
	"log"
	"fmt")

func getUsername(w http.ResponseWriter, r *http.Request) (username string) {
	username = ""
	user, err := aaa.CurrentUser(w, r)
	if err == nil {
        if err != nil {
        	return username
        }
        username = user.Username
	}
	return username
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	err := aaa.Login(w, r, username, password, "/")
	if err == nil {
		log.Println(username + " successfully logged in.")
		http.Redirect(w, r, "/", http.StatusSeeOther)		
	} else if err != nil && err.Error() == "httpauth: already authenticated" {
		log.Println(username + " already logged in.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
	} else {
		log.Println("LOGINHANDLER ERROR:")
		log.Println(err)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	err := aaa.Logout(w, r)
	if err != nil {
		fmt.Println(err)
		return
	} 
	log.Println("Logout")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func GuardPath(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := aaa.Authorize(w, r, true)
		if err != nil {
			fmt.Println(err)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		user, err := aaa.CurrentUser(w, r)
		if err == nil {
	        if err != nil {
	        	panic(err)
	        }
	        log.Println(user.Username + " is visiting " + r.Referer())
	        next.ServeHTTP(w, r)
		}
	})
}