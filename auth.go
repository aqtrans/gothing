package main

//Auth functions

import (
	"github.com/gorilla/securecookie"
	"github.com/zenazn/goji/web"
	"net/http"
	"log"
	"html/template"
	"time"
)
var cookieHandler = securecookie.New(
    securecookie.GenerateRandomKey(64),
    securecookie.GenerateRandomKey(32))

func SetSession(username string, w http.ResponseWriter, c web.C) {
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
    c.Env["user"] = username
}

func ClearSession(w http.ResponseWriter, c web.C) {
	cookie := &http.Cookie{
		Name:   "session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
    c.Env["user"] = nil
}

func GetUsername(r *http.Request, c web.C) (username string) {
	defer timeTrack(time.Now(), "GetUsername")
	if user, ok := c.Env["user"].(string); ok {
		username = user
	} else if cookie, err := r.Cookie("session"); err == nil {
		cookieValue := make(map[string]string)
		if err = cookieHandler.Decode("session", cookie.Value, &cookieValue); err == nil {
			username = cookieValue["user"]
			//log.Println(cookieValue)
		}
	} else {
		username = ""
	}
	log.Println("GetUsername: "+username)
	//log.Println(c.Env["user"])
	return username
}

func loginHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	username := template.HTMLEscapeString(r.FormValue("username"))
	password := template.HTMLEscapeString(r.FormValue("password"))
	log.Println("Referrer: "+ r.Referer())
	//log.Println(r.FormValue("username"))
	//log.Println(r.FormValue("password"))
	if username == cfg.Username && password == cfg.Password {
		SetSession(username, w, c)
		log.Println(username + " successfully logged in.")
		//log.Println(c.Env["user"])
		c.Env["msg"] = "Successfully Logged In"

		/*
		p, err := loadPage("Successfully Logged In", r, c)
		data := struct {
    		Page *Page
		    Title string
		} {
    		p,
    		"Successfully Logged In",
		}
		err = renderTemplate(w, "login.tmpl", data)
		if err != nil {
		    log.Println(err)
		    return
		}
		*/
		w.Write([]byte("success"))
	} else {
		log.Println("LOGINHANDLER ERROR:")
		c.Env["msg"] = "Login Error"
		/*
		p, err := loadPage("Login Error", r, c)
		data := struct {
    		Page *Page
		    Title string
		} {
    		p,
    		"Login Error",
		}
		err = renderTemplate(w, "login.tmpl", data)
		if err != nil {
		    log.Println(err)
		    return
		}
		*/
		w.Write([]byte("fail"))
	}
}

func logoutHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	ClearSession(w, c)
	log.Println("Logout")
	c.Env["msg"] = "Logged Out"
	p, err := loadPage("Logged out", r, c)
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

/*
func privHandler(c web.C, w http.ResponseWriter, r *http.Request) {
	err := aaa.Authorize(w, r, true)
	if err != nil {
		fmt.Println(err)
		//http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	user, err := aaa.CurrentUser(w, r)
	username := getUsername(c, w, r)
	if err == nil {
		p, err := loadPage("Please Login", username, c)
		data := struct {
    		Page *Page
		    User httpauth.UserData
		} {
    		p,
    		user,
		}
		t, err := template.New("priv").Parse(`
            <html>
            <head><title>Secret page</title></head>
            <body>
                <h1>Httpauth example<h1>
                {{ with .User }}
                    <h2>Hello {{ .Username }}</h2>
                    <p>Your role is '{{ .Role }}'. Your email is {{ .Email }}.</p>
                    <p>{{ if .Role | eq "admin" }}<a href="/admin">Admin page</a> {{ end }}<a href="/logout">Logout</a></p>
                {{ end }}
                <form action="/change" method="post" id="change">
                    <h3>Change email</h3>
                    <p><input type="email" name="new_email" placeholder="new email"></p>
                    <button type="submit">Submit</button>
                </form>
            </body>
            `)
        if err != nil {
        	panic(err)
        }
        t.Execute(w, data)
	}
}
*/

//Auth Handler for Goji
func AuthMiddleware(c *web.C, h http.Handler) http.Handler {
	handler := func(c web.C, w http.ResponseWriter, r *http.Request) {
		username := GetUsername(r, c)
		if username == "" {
			log.Println("AuthMiddleware mitigating: "+ r.Host + r.URL.String())
			c.Env["msg"] = "Please Login"
			p := &Page{
				TheName: "Smithers", 
				Title: "Please log in", 
				UN: "", 
				Msg: c.Env["msg"].(string),
			}
			data := struct {
	    		Page *Page
			    Title string
			} {
	    		p,
	    		"Please log in",
			}
			err := renderTemplate(w, "login.tmpl", data)
			if err != nil {
			    log.Println(err)
			    return
			}			
			return
		} else {
		    log.Println(username + " is visiting " + r.Referer())
		    h.ServeHTTP(w, r)			
		}
	}
	return web.HandlerFunc(handler)
}
