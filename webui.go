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
	"sort"
	"strconv"
	"strings"
	"sync"

	"appengine"

	"github.com/laktek/Stack-on-Go/stackongo"
)

type byCreationDate []stackongo.Question

func (a byCreationDate) Len() int           { return len(a) }
func (a byCreationDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byCreationDate) Less(i, j int) bool { return a[i].Creation_date > a[j].Creation_date }

type genReply struct {
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
	input, err := ioutil.ReadFile("3-12_dataset.json")
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	data.wrapper = new(stackongo.Questions)
	if err := json.Unmarshal(input, data.wrapper); err != nil {
		fmt.Println(err.Error())
		return
	}
	data.unansweredCache = data.wrapper.Items

	http.HandleFunc("/", handler)
	http.HandleFunc("/tag", tagHandler)
}

func handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		errorHandler(w, r, http.StatusNotFound, "")
		return
	}

	page := template.Must(template.ParseFiles("public/template.html"))

	c := appengine.NewContext(r)

	// TODO(gregoriou): Uncomment when ready to request from stackoverflow
	/*
		input, err := dataCollect.Collect(r)
		if err != nil {
		    errorHandler(w, r, http.StatusInternalServerError, err.Error())
			return
		}
	*/

	updatingCache_User(r)

	response := genReply{
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
		c.Criticalf("%v", err.Error())
	}
}

// Handler to find all questions with specific tags
func tagHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	tag := r.FormValue("q")
	tempData := webData{}

	for _, question := range data.unansweredCache {
		if contains(question.Tags, tag) {
			tempData.unansweredCache = append(tempData.unansweredCache, question)
		}
	}
	for _, question := range data.answeredCache {
		if contains(question.Tags, tag) {
			tempData.answeredCache = append(tempData.answeredCache, question)
		}
	}
	for _, question := range data.pendingCache {
		if contains(question.Tags, tag) {
			tempData.pendingCache = append(tempData.pendingCache, question)
		}
	}
	for _, question := range data.updatingCache {
		if contains(question.Tags, tag) {
			tempData.updatingCache = append(tempData.updatingCache, question)
		}
	}

	response := genReply{
		Wrapper: data.wrapper,
		Caches: []cacheInfo{
			cacheInfo{
				CacheType: "unanswered",
				Questions: tempData.unansweredCache,
				Info:      "These are questions that have not yet been answered by the Places API team",
			},
			cacheInfo{
				CacheType: "answered",
				Questions: tempData.answeredCache,
				Info:      "These are questions that have been answered by the Places API team",
			},
			cacheInfo{
				CacheType: "pending",
				Questions: tempData.pendingCache,
				Info:      "These are questions that are being answered by the Places API team",
			},
			cacheInfo{
				CacheType: "updating",
				Questions: tempData.updatingCache,
				Info:      "These are questions that will be answered in the next release",
			},
		},
		FindQuery: "",
	}

	page := template.Must(template.ParseFiles("public/tagTemplate.html"))
	if err := page.Execute(w, response); err != nil {
		fmt.Errorf("%v", err.Error())
	}
}

// updatings the caches based on input from the app
func updatingCache_User(r *http.Request) {
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
		case "unanswered":
			tempData.unansweredCache = append(tempData.unansweredCache, question)
		case "pending":
			tempData.pendingCache = append(tempData.pendingCache, question)
		case "updating":
			tempData.updatingCache = append(tempData.updatingCache, question)
		default:
			tempData.answeredCache = append(tempData.answeredCache, question)
		}
	}

	for i, question := range data.pendingCache {
		tag := "pending_state"
		tag = strings.Join([]string{tag, strconv.Itoa(i)}, "")
		form_input := r.PostFormValue(tag)
		switch form_input {
		case "unanswered":
			tempData.unansweredCache = append(tempData.unansweredCache, question)
		case "answered":
			tempData.answeredCache = append(tempData.answeredCache, question)
		case "updating":
			tempData.updatingCache = append(tempData.updatingCache, question)
		default:
			tempData.pendingCache = append(tempData.pendingCache, question)
		}
	}

	for i, question := range data.updatingCache {
		tag := "updating_state"
		tag = strings.Join([]string{tag, strconv.Itoa(i)}, "")
		form_input := r.PostFormValue(tag)
		switch form_input {
		case "unanswered":
			tempData.unansweredCache = append(tempData.unansweredCache, question)
		case "answered":
			tempData.answeredCache = append(tempData.answeredCache, question)
		case "pending":
			tempData.pendingCache = append(tempData.pendingCache, question)
		default:
			tempData.updatingCache = append(tempData.updatingCache, question)
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

func contains(slice []string, toFind string) bool {
	for _, tag := range slice {
		if strings.EqualFold(tag, toFind) {
			return true
		}
	}
	return false
}
