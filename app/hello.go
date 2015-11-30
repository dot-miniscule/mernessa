/*
	This Go Package responds to any request by sending a response containing the message Hello, vanessa.

*/

package hello

import (

	"dataCollect"
	"encoding/json"
	"fmt"

	"html/template"
	"net/http"

	"reflect"

	"github.com/laktek/Stack-on-Go/stackongo"
)

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

	 if err := page.Execute(w, nil); err != nil {
		panic(err)
	}
	input, err := dataCollect.Collect()
	if err != nil {
		fmt.Fprintf(w, "%v\n", err.Error())
		return
	}

	questions := new(stackongo.Questions)
	if err := json.Unmarshal(input, questions); err != nil {
		fmt.Fprintf(w, "%v\n", err.Error())
		return
	}

	fmt.Fprintf(w, "%v\n", reflect.TypeOf(questions.Items[0]))
	/*	for question := range questions.Items {
		fmt.Fprintf(w, "%v: %v\n", question.Title, question.Link)
	}*/
	fmt.Fprintf(w, "%v\n", questions.Quota_remaining)
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
