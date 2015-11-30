/*
	This Go Package responds to any request by sending a response containing the message Hello, vanessa.

*/

package hello

import (
	"dataCollect"
	"encoding/json"
	"fmt"
	"net/http"

	"reflect"

	"github.com/laktek/Stack-on-Go/stackongo"
)

func init() {
	http.HandleFunc("/", handler)
}

func handler(w http.ResponseWriter, r *http.Request) {
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
}
