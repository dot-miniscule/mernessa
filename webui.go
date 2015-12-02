/*
	This Go Package responds to any request by sending a response containing the message Hello, vanessa.

*/

package webui

import (
	"encoding/json"
	"html/template"
	"io/ioutil"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/laktek/Stack-on-Go/stackongo"
)

type byCreationDate []stackongo.Question

func (a byCreationDate) Len() int           { return len(a) }
func (a byCreationDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byCreationDate) Less(i, j int) bool { return a[i].Creation_date > a[j].Creation_date }

type reply struct {
	Wrapper   *stackongo.Questions
	Caches    []cacheInfo
	FindQuery string
}

type cacheInfo struct {
	CacheType string
	Questions []stackongo.Question
	Info      string
}

type webData struct {
	wrapper         *stackongo.Questions
	unansweredCache []stackongo.Question
	answeredCache   []stackongo.Question
	pendingCache    []stackongo.Question
	updatingCache   []stackongo.Question
	cacheLock       sync.Mutex
}

var data = webData{}

//The app engine will run its own main function and imports this code as a package
//So no main needs to be defined
//All routes go in to init
func init() {
	// TODO(gregoriou): Comment out when ready to request from stackoverflow
	input, err := ioutil.ReadFile("2-12_dataset.json")
	if err != nil {
		return
	}
	data.wrapper = new(stackongo.Questions)
	if err := json.Unmarshal(input, data.wrapper); err != nil {
		return
	}
	data.unansweredCache = data.wrapper.Items

	http.HandleFunc("/", handler)
}

func handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		errorHandler(w, r, http.StatusNotFound, "")
		return
	}

	page := template.Must(template.ParseFiles("public/template.html"))

	// TODO(gregoriou): Uncomment when ready to request from stackoverflow
	/*
		input, err := dataCollect.Collect(r)
		if err != nil {
			fmt.Fprintf(w, "%v\n", err.Error())
			return
		}
	*/

	data.updatingCache_User(r)

	response := reply{
		Wrapper: data.wrapper,
		Caches: []cacheInfo{
			cacheInfo{
				CacheType: "unanswered",
				Questions: data.unansweredCache,
				Info:      "These are questions that have not yet been answered by the Places API team",
			},
			cacheInfo{
				CacheType: "answered",
				Questions: data.answeredCache,
				Info:      "These are questions that have been answered by the Places API team",
			},
			cacheInfo{
				CacheType: "pending",
				Questions: data.pendingCache,
				Info:      "These are questions that are being answered by the Places API team",
			},
			cacheInfo{
				CacheType: "updating",
				Questions: data.updatingCache,
				Info:      "These are questions that will be answered in the next release",
			},
		},
		FindQuery: "",
	}
	if err := page.Execute(w, response); err != nil {
		panic(err)
	}
}

// updatings the caches based on input from the app
func (w webData) updatingCache_User(r *http.Request) {
	r.ParseForm()

	tempData := webData{}
	for i, question := range data.unansweredCache {
		tag := "unanswered_state"
		tag = strings.Join([]string{tag, strconv.Itoa(i)}, "")
		form_input := r.PostFormValue(tag)
		switch form_input {
		case "answered":
			tempData.answeredCache = append(tempData.answeredCache, question)
		case "pending":
			tempData.pendingCache = append(tempData.pendingCache, question)
		case "updating":
			tempData.updatingCache = append(tempData.updatingCache, question)
		default:
			tempData.unansweredCache = append(tempData.unansweredCache, question)
		}
	}

	for i, question := range data.answeredCache {
		tag := "answered_state"
		tag = strings.Join([]string{tag, strconv.Itoa(i)}, "")
		form_input := r.PostFormValue(tag)
		switch form_input {
		case "answered":
			tempData.answeredCache = append(tempData.answeredCache, question)
		case "pending":
			tempData.pendingCache = append(tempData.pendingCache, question)
		case "updating":
			tempData.updatingCache = append(tempData.updatingCache, question)
		default:
			tempData.unansweredCache = append(tempData.unansweredCache, question)
		}
	}

	for i, question := range data.pendingCache {
		tag := "pending_state"
		tag = strings.Join([]string{tag, strconv.Itoa(i)}, "")
		form_input := r.PostFormValue(tag)
		switch form_input {
		case "answered":
			tempData.answeredCache = append(tempData.answeredCache, question)
		case "pending":
			tempData.pendingCache = append(tempData.pendingCache, question)
		case "updating":
			tempData.updatingCache = append(tempData.updatingCache, question)
		default:
			tempData.unansweredCache = append(tempData.unansweredCache, question)
		}
	}

	for i, question := range data.updatingCache {
		tag := "updating_state"
		tag = strings.Join([]string{tag, strconv.Itoa(i)}, "")
		form_input := r.PostFormValue(tag)
		switch form_input {
		case "answered":
			tempData.answeredCache = append(tempData.answeredCache, question)
		case "pending":
			tempData.pendingCache = append(tempData.pendingCache, question)
		case "updating":
			tempData.updatingCache = append(tempData.updatingCache, question)
		default:
			tempData.unansweredCache = append(tempData.unansweredCache, question)
		}
	}

	sort.Stable(byCreationDate(tempData.unansweredCache))
	sort.Stable(byCreationDate(tempData.answeredCache))
	sort.Stable(byCreationDate(tempData.pendingCache))
	sort.Stable(byCreationDate(tempData.updatingCache))

	data.unansweredCache = tempData.unansweredCache
	data.answeredCache = tempData.answeredCache
	data.pendingCache = tempData.pendingCache
	data.updatingCache = tempData.updatingCache
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
