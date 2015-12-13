/*
	This Go Package responds to any request by sending a response containing the message Hello, vanessa.

*/

package webui

import (
	"backend"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"appengine"

	"github.com/laktek/Stack-on-Go/stackongo"
)

// Functions for sorting
type byCreationDate []stackongo.Question

func (a byCreationDate) Len() int           { return len(a) }
func (a byCreationDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byCreationDate) Less(i, j int) bool { return a[i].Creation_date > a[j].Creation_date }

// Reply to send to template
type genReply struct {
	Wrapper   *stackongo.Questions
	Caches    []cacheInfo
	User      stackongo.User
	Qns       map[int]stackongo.User
	FindQuery string
}

// Info on the various caches
type cacheInfo struct {
	CacheType string               // "unanswered"/"answered"/"pending"/"updating"
	Questions []stackongo.Question // list of questions
	Info      string               // blurb about the cache
}

type webData struct {
	wrapper         *stackongo.Questions // Request information
	unansweredCache []stackongo.Question
	answeredCache   []stackongo.Question
	pendingCache    []stackongo.Question
	updatingCache   []stackongo.Question
	cacheLock       sync.Mutex // For multithreading, will use to avoid updating cache and serving cache at the same time
}

type userData struct {
	user_info     stackongo.User
	access_token  string
	answeredCache []stackongo.Question
	pendingCache  []stackongo.Question
	updatingCache []stackongo.Question
}

// Global variable with cache info
var data = webData{}
var pageData = webData{}
var users = make(map[int]*userData)
var qns = make(map[int]stackongo.User)
var guest = stackongo.User{
	Display_name: "guest",
}

//The app engine will run its own main function and imports this code as a package
//So no main needs to be defined
//All routes go in to init
func init() {
	// TODO(gregoriou): Comment out when ready to request from stackoverflow
	input, err := ioutil.ReadFile("3-12_dataset.json") // Read from most recent file
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	db := backend.SqlInit()

	pageData.wrapper = new(stackongo.Questions) // Create a new wrapper
	if err := json.Unmarshal(input, pageData.wrapper); err != nil {
		fmt.Println(err.Error())
		return
	}
	//Comment Out the next line to avoid ridiculous loading times
	//pageData.unansweredCache = pageData.wrapper.Items // At start, all questions are unanswered

	//Iterate through each question returned, and add it to the database.
	for _, item := range pageData.unansweredCache {
		//INSERT IGNORE ensures that the same question won't be added again
		//This will probably need to change as we better develop the workflow from local to stack exchange.
		stmt, err := db.Prepare("INSERT IGNORE INTO questions(question_id, question_title, question_URL) VALUES (?, ?, ?)")
		if err != nil {
			log.Fatal(err)
		}

		_, err = stmt.Exec(item.Question_id, item.Title, item.Link)
		if err != nil {
			log.Fatal("Insertion failed of question failed:\t", err)
		}

		//Need to add the tags in to the database as well, and ensure that they are joined to the questions
		//Use a secondary mapping table to store the relationship between tags and questions.
		//TODO: Complete mapping of tags to questions
		for _, tag := range item.Tags {
			stmt, err = db.Prepare("INSERT IGNORE INTO tags(tag) VALUES (?)")

			if err != nil {
				log.Fatal(err)
			}

			_, err = stmt.Exec(tag)
			if err != nil {
				log.Fatal("Insertion of tag \"", tag, "\" failed:", err)
			}
		}
	}

	log.Println("New records added successfully!")

	//Reading from database
	var (
		url     string
		title   string
		id      int
		updated int
		state   string
	)
	rows, err := db.Query("select * from questions")
	if err != nil {
		log.Fatal("query failed:\t", err)
	}

	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&id, &title, &url, &updated, &state)
		if err != nil {
			log.Fatal(err)
		}
		currentQ := stackongo.Question{
			Question_id: id,
			Title:       title,
			Link:        url,
		}

		//Switch on the state as read from the database to ensure question is added to correct cace
		switch state {
		case "unanswered":
			data.unansweredCache = append(data.unansweredCache, currentQ)
		case "answered":
			data.unansweredCache = append(data.answeredCache, currentQ)
		case "pending":
			data.pendingCache = append(data.pendingCache, currentQ)
		case "updating":
			data.updatingCache = append(data.updatingCache, currentQ)

		}
	}

	http.HandleFunc("/", handler)
	http.HandleFunc("/home", mainHandler)
	http.HandleFunc("/tag", mainHandler)
	http.HandleFunc("/user", mainHandler)

}

func handler(w http.ResponseWriter, r *http.Request) {
	auth_url := backend.AuthURL()
	header := w.Header()
	header.Add("Location", auth_url)
	w.WriteHeader(302)
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	// Create a new appengine context for logging purposes
	c := appengine.NewContext(r)

	backend.SetTransport(r)
	_ = backend.NewSession(r)

	code := r.URL.Query().Get("code")

	access_tokens, err := backend.ObtainAccessToken(code)
	if err == nil {
		http.SetCookie(w, &http.Cookie{Name: "access_token", Value: access_tokens["access_token"]})
	}

	token, err := r.Cookie("access_token")
	if err != nil {
		handler(w, r)
		return
	}

	user, err := backend.AuthenticatedUser(map[string]string{}, token.Value)
	if err != nil {
		c.Errorf(err.Error())
		return
	}

	if _, ok := users[user.User_id]; !ok {
		users[user.User_id] = &userData{}
		users[user.User_id].init(user, token.Value)
	}

	// TODO(gregoriou): Uncomment when ready to request from stackoverflow
	/*
		input, err := dataCollect.Collect(r)
		if err != nil {
		    errorHandler(w, r, http.StatusInternalServerError, err.Error())
			return
		}
	*/

	// update the new cache at refresh
	updatingCache_User(r, c, user)

	// Send to tag subpage
	if r.URL.Path == "/tag" && r.FormValue("q") != "" {
		tagHandler(w, r, c, user)
		return
	}

	// Send to user subpage
	if r.URL.Path == "/user" {
		userHandler(w, r, c, user)
		return
	}
	/*
		if r.URL.Path == "/login" {
			loginHandler(w, r, c)
			return
		}
	*/
	page := template.Must(template.ParseFiles("public/template.html"))
	// WriteResponse creates a new response with the various caches
	if err := page.Execute(w, writeResponse(user, data.unansweredCache, data.answeredCache, data.pendingCache, data.updatingCache)); err != nil {
		c.Criticalf("%v", err.Error())
	}
}

// Handler to find all questions with specific tags
func tagHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, user stackongo.User) {
	// Collect query
	tag := r.FormValue("q")

	// Create and fill in a new webData struct
	tempData := webData{}

	// range through the question caches golang stackongoand add if the question contains the tag
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

	page := template.Must(template.ParseFiles("public/template.html"))
	if err := page.Execute(w, writeResponse(user, tempData.unansweredCache, tempData.answeredCache, tempData.pendingCache, tempData.updatingCache)); err != nil {
		c.Criticalf("%v", err.Error())
	}
}

func userHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, user stackongo.User) {
	userID, _ := strconv.Atoi(r.FormValue("id"))

	page := template.Must(template.ParseFiles("public/template.html"))

	if _, ok := users[userID]; !ok {
		page.Execute(w, writeResponse(user, nil, nil, nil, nil))
		return
	}
	if err := page.Execute(w, writeResponse(user, nil, users[userID].answeredCache, users[userID].pendingCache, users[userID].updatingCache)); err != nil {
		c.Criticalf("%v", err.Error())
	}
}

// Write a genReply struct with the inputted Question slices
func writeResponse(user stackongo.User, unanswered []stackongo.Question, answered []stackongo.Question, pending []stackongo.Question, updating []stackongo.Question) genReply {
	return genReply{
		Wrapper: pageData.wrapper, // The global wrapper
		Caches: []cacheInfo{ // Slices caches and their relevant info
			cacheInfo{
				CacheType: "unanswered",
				Questions: unanswered,
				Info:      "These are questions that have not yet been answered by the Places API team",
			},
			cacheInfo{
				CacheType: "answered",
				Questions: answered,
				Info:      "These are questions that have been answered by the Places API team",
			},
			cacheInfo{
				CacheType: "pending",
				Questions: pending,
				Info:      "These are questions that are being answered by the Places API team",
			},
			cacheInfo{
				CacheType: "updating",
				Questions: updating,
				Info:      "These are questions that will be answered in the next release",
			},
		},
		User:      user,
		FindQuery: "",
	}
}

// updating the caches based on input from the app
func updatingCache_User(r *http.Request, c appengine.Context, user stackongo.User) {
	// required to collect post form data
	r.ParseForm()

	if _, ok := users[user.User_id]; !ok {
		users[user.User_id] = &userData{}
		userInfo, err := backend.GetUser(user.User_id, map[string]string{})
		if err != nil {
			c.Errorf(err.Error())
			return
		}
		users[user.User_id].init(userInfo, "")
	}

	tempData := webData{}

	// Collect the submitted form info based on the name of the form
	for i, question := range data.unansweredCache {
		name := "unanswered_state"
		name = strings.Join([]string{name, strconv.Itoa(i)}, "")
		form_input := r.PostFormValue(name)
		switch form_input {
		case "answered":
			tempData.answeredCache = append(tempData.answeredCache, question)
			users[user.User_id].answeredCache = append(users[user.User_id].answeredCache, question)
		case "pending":
			tempData.pendingCache = append(tempData.pendingCache, question)
			users[user.User_id].pendingCache = append(users[user.User_id].pendingCache, question)
		case "updating":
			tempData.updatingCache = append(tempData.updatingCache, question)
			users[user.User_id].updatingCache = append(users[user.User_id].updatingCache, question)
		default:
			tempData.unansweredCache = append(tempData.unansweredCache, question)
		}
		if form_input != "" && form_input != "unanswered" {
			qns[question.Question_id] = user
		}
	}

	for i, question := range data.answeredCache {
		name := "answered_state"
		name = strings.Join([]string{name, strconv.Itoa(i)}, "")
		form_input := r.PostFormValue(name)
		switch form_input {
		case "unanswered":
			tempData.unansweredCache = append(tempData.unansweredCache, question)
		case "answered":
			tempData.answeredCache = append(tempData.answeredCache, question)
			users[user.User_id].answeredCache = append(users[user.User_id].answeredCache, question)
		case "pending":
			tempData.pendingCache = append(tempData.pendingCache, question)
			users[user.User_id].pendingCache = append(users[user.User_id].pendingCache, question)
		case "updating":
			tempData.updatingCache = append(tempData.updatingCache, question)
			users[user.User_id].updatingCache = append(users[user.User_id].updatingCache, question)
		default:
			tempData.answeredCache = append(tempData.answeredCache, question)
		}
		editor := qns[question.Question_id]

		for i, q := range users[editor.User_id].answeredCache {
			if question.Question_id == q.Question_id {
				users[editor.User_id].answeredCache = append(users[editor.User_id].answeredCache[:i], users[editor.User_id].answeredCache[i+1:]...)
			}
		}
		if form_input == "unanswered" {
			qns[question.Question_id] = stackongo.User{}
			delete(qns, question.Question_id)
		} else if form_input != "" {
			qns[question.Question_id] = user
		}
	}

	for i, question := range data.pendingCache {
		name := "pending_state"
		name = strings.Join([]string{name, strconv.Itoa(i)}, "")
		form_input := r.PostFormValue(name)
		switch form_input {
		case "unanswered":
			tempData.unansweredCache = append(tempData.unansweredCache, question)
		case "answered":
			tempData.answeredCache = append(tempData.answeredCache, question)
			users[user.User_id].answeredCache = append(users[user.User_id].answeredCache, question)
		case "pending":
			tempData.pendingCache = append(tempData.pendingCache, question)
			users[user.User_id].pendingCache = append(users[user.User_id].pendingCache, question)
		case "updating":
			tempData.updatingCache = append(tempData.updatingCache, question)
			users[user.User_id].updatingCache = append(users[user.User_id].updatingCache, question)
		default:
			tempData.pendingCache = append(tempData.pendingCache, question)
		}
		editor := qns[question.Question_id]

		for i, q := range users[editor.User_id].pendingCache {
			if question.Question_id == q.Question_id {
				users[editor.User_id].pendingCache = append(users[editor.User_id].pendingCache[:i], users[editor.User_id].pendingCache[i+1:]...)
			}
		}
		if form_input == "unanswered" {
			qns[question.Question_id] = stackongo.User{}
			delete(qns, question.Question_id)
		} else if form_input != "" {
			qns[question.Question_id] = user
		}
	}

	for i, question := range data.updatingCache {
		name := "updating_state"
		name = strings.Join([]string{name, strconv.Itoa(i)}, "")
		form_input := r.PostFormValue(name)
		switch form_input {
		case "unanswered":
			tempData.unansweredCache = append(tempData.unansweredCache, question)
		case "answered":
			tempData.answeredCache = append(tempData.answeredCache, question)
			users[user.User_id].answeredCache = append(users[user.User_id].answeredCache, question)
		case "pending":
			tempData.pendingCache = append(tempData.pendingCache, question)
			users[user.User_id].pendingCache = append(users[user.User_id].pendingCache, question)
		case "updating":
			tempData.updatingCache = append(tempData.updatingCache, question)
			users[user.User_id].updatingCache = append(users[user.User_id].updatingCache, question)
		default:
			tempData.updatingCache = append(tempData.updatingCache, question)
		}
		editor := qns[question.Question_id]

		for i, q := range users[editor.User_id].updatingCache {
			if question.Question_id == q.Question_id {
				users[editor.User_id].updatingCache = append(users[editor.User_id].updatingCache[:i], users[editor.User_id].updatingCache[i+1:]...)
			}
		}
		if form_input == "unanswered" {
			qns[question.Question_id] = stackongo.User{}
			delete(qns, question.Question_id)
		} else if form_input != "" {
			qns[question.Question_id] = user
		}
	}

	// sort slices by creation date
	sort.Stable(byCreationDate(tempData.unansweredCache))
	sort.Stable(byCreationDate(tempData.answeredCache))
	sort.Stable(byCreationDate(tempData.pendingCache))
	sort.Stable(byCreationDate(tempData.updatingCache))

	// replace global caches with new caches
	data.unansweredCache = tempData.unansweredCache
	data.answeredCache = tempData.answeredCache
	data.pendingCache = tempData.pendingCache
	data.updatingCache = tempData.updatingCache

	sort.Stable(byCreationDate(users[user.User_id].answeredCache))
	sort.Stable(byCreationDate(users[user.User_id].pendingCache))
	sort.Stable(byCreationDate(users[user.User_id].updatingCache))
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

// Returns true if toFind is an element of slice
func contains(slice []string, toFind string) bool {
	for _, tag := range slice {
		if reflect.DeepEqual(tag, toFind) {
			return true
		}
	}
	return false
}

func (user userData) init(u stackongo.User, token string) {
	user.user_info = u
	user.access_token = token
	user.answeredCache = []stackongo.Question{}
	user.pendingCache = []stackongo.Question{}
	user.updatingCache = []stackongo.Question{}
}
