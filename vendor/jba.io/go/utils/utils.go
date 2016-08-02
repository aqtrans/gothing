package utils

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"expvar"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const timestamp = "2006-01-02 at 03:04:05PM"

var (
	Debug         bool
	eResCount     *expvar.Map
	ExpIndexC     *expvar.Int
	eFileUploads  *expvar.Int
	eImageUploads *expvar.Int
	startTime     = time.Now().UTC()
)

//JSON Response
type jsonresponse struct {
	Name    string `json:"name,omitempty"`
	Success bool   `json:"success"`
}

func init() {
	// Additional expvars
	//expvar.Publish("Goroutines",expvar.Func(expGoroutines))
	expvar.Publish("Uptime", expvar.Func(expUptime))
	ExpIndexC = expvar.NewInt("index_hits")
	eResCount = expvar.NewMap("response_counts").Init()
	//eResCount.Set("200", expvar.NewInt("200"))
	//eResCount.Set("302", expvar.NewInt("302"))
	//eResCount.Set("400", expvar.NewInt("400"))
	//eResCount.Set("404", expvar.NewInt("404"))
	//eResCount.Set("500", expvar.NewInt("500"))
}

/*func expGoroutines() interface{} {
	return runtime.NumGoroutine()
}*/

// uptime is an expvar.Func compliant wrapper for uptime info.
func expUptime() interface{} {
	now := time.Now().UTC()
	uptime := now.Sub(startTime)
	return map[string]interface{}{
		"start_time":  startTime,
		"uptime":      uptime.String(),
		"uptime_ms":   fmt.Sprintf("%d", uptime.Nanoseconds()/1000000),
		"server_time": now,
	}
}

// HandleExpvars is yanked from: https://github.com/meatballhat/expvarplus/blob/master/expvarplus.go
// HandleExpvars does the same thing as the private expvar.expvarHandler, but
// exposed as public for pluggability into other web frameworks and generates
// json in a maybe slightly kinda more sane way (???).
func HandleExpvars(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	vars := map[string]interface{}{}

	expvar.Do(func(kv expvar.KeyValue) {
		var unStrd interface{}
		json.Unmarshal([]byte(kv.Value.String()), &unStrd)
		vars[kv.Key] = unStrd
	})

	jsonBytes, err := json.MarshalIndent(vars, "", "    ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error":%q}`, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, string(jsonBytes)+"\n")
}

func Debugln(v ...interface{}) {
	if Debug {
		var buf bytes.Buffer
		debuglogger := log.New(&buf, "Debug: ", log.Ltime)
		debuglogger.SetOutput(os.Stderr)
		debuglogger.Print(v)

		//d := log.New(os.Stdout, "DEBUG: ", log.Ldate)
		//d.Println(v)

		//fmt.Println(v)
	}
}

func PrettyDate(date int64) string {
	if date == 0 {
		return "N/A"
	}
	t := time.Unix(date, 0)
	return t.Format(timestamp)
}

func ImgClass(s string) string {
	if strings.HasSuffix(s, ".gif") {
		return "gifs"
	}
	if strings.HasSuffix(s, ".webm") {
		return "gifs"
	}
	return "imgs"
}

func ImgExt(s string) string {
	if strings.HasSuffix(s, ".gif") {
		return "gif"
	}
	if strings.HasSuffix(s, ".webm") {
		return "webm"
	}
	return ""
}

//SafeHTML is a template function to ensure HTML isn't escaped
func SafeHTML(s string) template.HTML {
	return template.HTML(s)
}

//GetScheme is a hack to allow me to make full URLs due to absence of http:// from URL.Scheme in dev situations
//When behind Nginx, use X-Forwarded-Proto header to retrieve this, then just tack on "://"
//getScheme(r) should return http:// or https://
func GetScheme(r *http.Request) (scheme string) {
	scheme = r.Header.Get("X-Forwarded-Proto") + "://"
	/*
		scheme = "http://"
		if r.TLS != nil {
			scheme = "https://"
		}
	*/
	if scheme == "://" {
		scheme = "http://"
	}
	return scheme
}

//TimeTrack is a simple function to time the duration of any function you wish
// Example (at the beginning of a function you wish to time): defer utils.TimeTrack(time.Now(), "[func name]")
func TimeTrack(start time.Time, name string) {
	if Debug {
		elapsed := time.Since(start)
		//log.Printf("[timer] %s took %s", name, elapsed)

		var buf bytes.Buffer
		timerlogger := log.New(&buf, "Timer: ", log.Ltime)
		timerlogger.SetOutput(os.Stderr)
		timerlogger.Printf("[timer] %s took %s", name, elapsed)
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Status() int {
	return w.status
}

func (w *statusWriter) Size() int {
	return w.size
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	written, err := w.ResponseWriter.Write(b)
	w.size += written
	return written, err
}

//Logger is my custom logging middleware
// It prints all HTTP requests to a file called http.log, as well as helps the expvarHandler log the status codes
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		//Log to file
		f, err := os.OpenFile("./http.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		defer f.Close()

		start := time.Now()
		writer := statusWriter{w, 0, 0}

		buf.WriteString("Started ")
		fmt.Fprintf(&buf, "%s ", r.Method)
		fmt.Fprintf(&buf, "%q ", r.URL.String())
		fmt.Fprintf(&buf, "|Host: %s |RawURL: %s |UserAgent: %s |Scheme: %s |IP: %s ", r.Host, r.Header.Get("X-Raw-URL"), r.Header.Get("User-Agent"), GetScheme(r), r.Header.Get("X-Forwarded-For"))
		buf.WriteString("from ")
		buf.WriteString(r.RemoteAddr)

		//log.SetOutput(io.MultiWriter(os.Stdout, f))
		toplogger := log.New(&buf, "HTTP: ", log.LstdFlags)
		toplogger.SetOutput(f)
		toplogger.Print(buf.String())
		Debugln(buf.String())

		//Reset buffer to be reused by the end stuff
		buf.Reset()

		next.ServeHTTP(&writer, r)

		end := time.Now()
		latency := end.Sub(start)
		status := writer.Status()

		buf.WriteString("Returning ")
		fmt.Fprintf(&buf, "%v", status)
		buf.WriteString(" for ")
		fmt.Fprintf(&buf, "%q ", r.URL.String())
		buf.WriteString(" in ")
		fmt.Fprintf(&buf, "%s", latency)
		//log.SetOutput(io.MultiWriter(os.Stdout, f))

		bottomlogger := log.New(&buf, "HTTP: ", log.LstdFlags)
		bottomlogger.SetOutput(f)
		bottomlogger.Print(buf.String())
		Debugln(buf.String())

		// Log status code to expvar
		logStatusCode(status)

	})
}

//logStatusCode takes the HTTP status code from above, and tosses it into an expvar map
func logStatusCode(c int) {
	//log.Println(c)
	cstring := strconv.Itoa(c)
	//log.Println(cstring)
	// From testing, it appears expvar's Map Add() function will
	//  happily create new Keys if they do not already exist!
	eResCount.Add(cstring, 1)
}

//RandKey generates a random key of specific length
func RandKey(leng int8) string {
	dictionary := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	rb := make([]byte, leng)
	rand.Read(rb)
	for k, v := range rb {
		rb[k] = dictionary[v%byte(len(dictionary))]
	}
	sessID := string(rb)
	return sessID
}

//makeJSON cooks up formatted JSON given a glob of data
func makeJSON(w http.ResponseWriter, data interface{}) ([]byte, error) {
	jsonData, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return nil, err
	}
	Debugln(string(jsonData))
	return jsonData, nil
}

//WriteJ writes a json-formatted response containing a name of an item being worked on,
// and the success of the function performed on it
// TODO: Probably rework this and it's associated makeJSON func, so this function is generalized
//       and the makeJSON func is specific to my jsonresponse struct
func WriteJ(w http.ResponseWriter, name string, success bool) error {
	j := jsonresponse{
		Name:    name,
		Success: success,
	}
	json, err := makeJSON(w, j)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	w.Write(json)
	Debugln(string(json))
	return nil
}

//ServeContent checks for file existence, and if there, serves it so it can be cached
func ServeContent(w http.ResponseWriter, r *http.Request, dir, file string) {
	f, err := http.Dir(dir).Open(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	content := io.ReadSeeker(f)
	http.ServeContent(w, r, file, time.Now(), content)
	return
}

// Taken from http://reinbach.com/golang-webapps-1.html
func StaticHandler(w http.ResponseWriter, r *http.Request) {
	staticFile := r.URL.Path[len("/assets/"):]

	defer TimeTrack(time.Now(), "StaticHandler "+staticFile)

	//log.Println(staticFile)
	if len(staticFile) != 0 {
		/*
		   f, err := http.Dir("assets/").Open(staticFile)
		   if err == nil {
		       content := io.ReadSeeker(f)
		       http.ServeContent(w, r, staticFile, time.Now(), content)
		       return
		   }*/
		ServeContent(w, r, "assets/", staticFile)
		return
	}
	http.NotFound(w, r)
}

func FaviconHandler(w http.ResponseWriter, r *http.Request) {
	//log.Println(r.URL.Path)
	if r.URL.Path == "/favicon.ico" {
		ServeContent(w, r, "assets/", "/favicon.ico")
		return
	} else if r.URL.Path == "/favicon.png" {
		ServeContent(w, r, "assets/", "/favicon.png")
		return
	} else {
		http.NotFound(w, r)
		return
	}

}

func RobotsHandler(w http.ResponseWriter, r *http.Request) {
	//log.Println(r.URL.Path)
	if r.URL.Path == "/robots.txt" {
		ServeContent(w, r, "assets/", "/robots.txt")
		return
	}
	http.NotFound(w, r)
}
