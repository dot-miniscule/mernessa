/*
	This Go Package responds to any request by sending a response containing the message Hello, vanessa.

*/

package hello

import (
<<<<<<< HEAD
	"dataCollect"
	"encoding/json"
	"fmt"
=======
	"html/template"
>>>>>>> 1f4e34ba3f9aca6b3cef4ec597d38381feb4ac5a
	"net/http"

<<<<<<< HEAD
	"reflect"

	"github.com/laktek/Stack-on-Go/stackongo"
)

=======
//The app engine will run its own main function and imports this code as a package
//So no main needs to be defined
//All routes go in to init
>>>>>>> 1f4e34ba3f9aca6b3cef4ec597d38381feb4ac5a
func init() {
	http.HandleFunc("/", handler)
}

func handler(w http.ResponseWriter, r *http.Request) {
<<<<<<< HEAD
	fmt.Fprintf(w, "Hello, Vanessa!")
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
=======
	if r.URL.Path != "/" {
		errorHandler(w, r, http.StatusNotFound, "")
		return
	}

	page := template.Must(template.ParseFiles("public/template.html"))

	 if err := page.Execute(w, nil); err != nil {
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
>>>>>>> 1f4e34ba3f9aca6b3cef4ec597d38381feb4ac5a
}
