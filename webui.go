/*
	This Go Package responds to any request by sending a response containing the message Hello, vanessa.

*/

package webui

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/laktek/Stack-on-Go/stackongo"
)

type findReply struct {
	Questions *stackongo.Questions
}

type reply struct {
	Reply     *stackongo.Questions
	FindQuery string
}

type webData struct {
	cache     *findReply
	cacheLock sync.Mutex
}

type WebServer struct {
	Port int
	Path string
	Tmpl *template.Template
	Data *webData
}

//The app engine will run its own main function and imports this code as a package
//So no main needs to be defined
//All routes go in to init
func init() {
	http.HandleFunc("/", handler)
}

func handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		errorHandler(w, r, http.StatusNotFound, "")
		return
	}

	page := template.Must(template.ParseFiles("public/template.html"))
	/*
		input, err := dataCollect.Collect(r)
		if err != nil {
			fmt.Fprintf(w, "%v\n", err.Error())
			return
		}
	*/
	input, err := ioutil.ReadFile("27-11_dataset.json")
	if err != nil {
		fmt.Fprintf(w, "%v", err.Error())
		return
	}

	questions := new(stackongo.Questions)
	if err := json.Unmarshal(input, questions); err != nil {
		fmt.Fprintf(w, "%v", err.Error())
		return
	}

	response := reply{
		Reply:     questions,
		FindQuery: "",
	}
	if err := page.Execute(w, response); err != nil {
		panic(err)
	}
}

func errorHandler(w http.ResponseWriter, r *http.Request, status int, err string) {
	w.WriteHeader(status)
	switch status {
	case http.StatusNotFound:
		page := template.Must(template.ParseFiles("public/404.html"))
		if err := page.Execute(w, nil); err != nil {
			errorHandler(w, r, http.StatusInternalServerError, err.Error())
			return
		}
	}
}
