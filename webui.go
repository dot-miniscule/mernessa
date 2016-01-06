/*

 */

package webui

import (
	"backend"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"sync"
	"time"

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
	Wrapper    *stackongo.Questions   // Information about the query
	Caches     []cacheInfo            // Slice of the 4 caches (Unanswered, Answered, Pending, Updating)
	User       stackongo.User         // Information on the current user
	Qns        map[int]stackongo.User // Map of users by question ids
	UpdateTime int32
	Query      string
}

// Info on the various caches
type cacheInfo struct {
	CacheType string               // "unanswered"/"answered"/"pending"/"updating"
	Questions []stackongo.Question // list of questions
	Info      string               // blurb about the cache
}

type webData struct {
	Wrapper         *stackongo.Questions   // Request information
	UnansweredCache []stackongo.Question   // Unanswered questions
	AnsweredCache   []stackongo.Question   // Answered questions
	PendingCache    []stackongo.Question   // Pending questions
	UpdatingCache   []stackongo.Question   // Updating questions
	Qns             map[int]stackongo.User // Map of users by question ids
	CacheLock       sync.Mutex             // For multithreading, will use to avoid updating cache and serving cache at the same time
}

type userData struct {
	User_info     stackongo.User       // SE user info
	access_token  string               // Token to access info
	answeredCache []stackongo.Question //  answered by user
	pendingCache  []stackongo.Question // Questions being answered by user
	updatingCache []stackongo.Question // Questions that are being updated
}

type tagData struct {
	Tag   string //The actual tag, hyphenated string
	Count int    //The number of questions with that tag in the db
}

type userInfo struct {
	ID   int
	Name string
	Pic  string
	Link string
}

func newWebData() webData {
	return webData{
		UnansweredCache: []stackongo.Question{},
		AnsweredCache:   []stackongo.Question{},
		PendingCache:    []stackongo.Question{},
		UpdatingCache:   []stackongo.Question{},
		Qns:             make(map[int]stackongo.User),
	}
}

const timeout = 1 * time.Minute

// Global variable with cache info
var data = newWebData()

// Map of users by user ids
var users = make(map[int]*userData)

// Standard guest user
var guest = stackongo.User{
	Display_name: "Guest",
}

// Pointer to database connection to communicate with Cloud SQL
var db = backend.SqlInit()

//Stores the last time the database was read into the cache
//This is then checked against the update time of the database and determine whether the cache should be updated
var mostRecentUpdate int32

// Functions for template to recieve data from maps
func (r genReply) GetUserIDByQn(id int) int {
	return r.Qns[id].User_id
}
func (r genReply) GetUserNameByQn(id int) string {
	return r.Qns[id].Display_name
}
func (r genReply) CacheUpdated() bool {
	return mostRecentUpdate > r.UpdateTime
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

	//Read questions from Stack wrapper
	data.Wrapper = new(stackongo.Questions) // Create a new wrapper
	if err := json.Unmarshal(input, data.Wrapper); err != nil {
		fmt.Println(err.Error())
		return
	}

	// Initialising stackongo session
	backend.NewSession()

	day := 24 * time.Hour
	week := 7 * day
	toDate := time.Now()
	fromDate := toDate.Add(-1*week + day)

	go func(fromDate time.Time, toDate time.Time, db *sql.DB) {
		for i := 0; i < 0; i++ {
			reply, err := backend.GetNewQns(fromDate, toDate)
			if err != nil {
				log.Printf("Error getting new questions: %v", err.Error())
				continue
			}
			if err = backend.AddQuestions(db, reply); err != nil {
				log.Printf("Error updating database: %v", err.Error())
				continue
			}
			toDate = fromDate.Add(-1 * day)
			fromDate = toDate.Add(-1*week + day)
		}
	}(fromDate, toDate, db)

	log.Println("Initial cache download")
	refreshCache()

	count := 1
	go func(count int) {
		for {
			if checkDBUpdateTime("questions", mostRecentUpdate) {
				log.Printf("Refreshing cache %v", count)
				refreshCache()
				count++
			}
		}
	}(count)

	http.HandleFunc("/login", authHandler)
	http.HandleFunc("/", handler)
	http.HandleFunc("/tag", handler)
	http.HandleFunc("/user", handler)
	http.HandleFunc("/viewTags", handler)
	http.HandleFunc("/viewUsers", handler)
	http.HandleFunc("/userPage", handler)
	http.HandleFunc("/dbUpdated", updateHandler)
}

// Handler for authorizing user
func authHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Redirecting to SO login")
	auth_url := backend.AuthURL()
	header := w.Header()
	header.Add("Location", auth_url)
	w.WriteHeader(302)
}

func updateHandler(w http.ResponseWriter, r *http.Request) {

	time, _ := strconv.Atoi(r.FormValue("time"))
	page, _ := template.New("updatePage").Parse("{{$.CacheUpdated}}")
	// WriteResponse creates a new response with the various caches
	if err := page.Execute(w, genReply{UpdateTime: int32(time)}); err != nil {
		log.Printf("%v", err.Error())
	}
}

// Handler for main information to be read and written from
func handler(w http.ResponseWriter, r *http.Request) {
	// Create a new appengine context for logging purposes
	c := appengine.NewContext(r)

	backend.SetTransport(r)
	backend.NewSession()

	user := getUser(w, r, c)

	page := template.Must(template.ParseFiles("public/template.html"))
	// update the new cache on submit
	cookie, _ := r.Cookie("submitting")
	if cookie != nil && cookie.Value == "true" {
		err := updatingCache_User(r, c, user)
		if err != nil {
			c.Errorf(err.Error())
		}
		http.SetCookie(w, &http.Cookie{Name: "submitting", Value: ""})
	}

	// Send to tag subpage
	if r.URL.Path == "/tag" && r.FormValue("tagSearch") != "" {
		tagHandler(w, r, c, user)
		return
	}

	// Send to user subpage
	if r.URL.Path == "/user" {
		userHandler(w, r, c, user)
		return
	}

	//Send to viewTags page
	if r.URL.Path == "/viewTags" {
		viewTagsHandler(w, r, c, user)
		return
	}

	//Send to viewUsers page
	if r.URL.Path == "/viewUsers" {
		viewUsersHandler(w, r, c, user)
		return
	}

	if r.URL.Path == "/userPage" {
		userPageHandler(w, r, c, user)
		return
	}

	// WriteResponse creates a new response with the various caches
	if err := page.Execute(w, writeResponse(user, data, c, "")); err != nil {
		c.Criticalf("%v", err.Error())
	}
}

// Handler to find all questions with specific tags
func tagHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, user stackongo.User) {
	// Collect query
	tag := r.FormValue("tagSearch")

	// Create and fill in a new webData struct
	tempData := webData{}

	data.CacheLock.Lock()
	// range through the question caches golang stackongoand add if the question contains the tag
	for _, question := range data.UnansweredCache {
		if contains(question.Tags, tag) {
			tempData.UnansweredCache = append(tempData.UnansweredCache, question)
		}
	}
	for _, question := range data.AnsweredCache {
		if contains(question.Tags, tag) {
			tempData.AnsweredCache = append(tempData.AnsweredCache, question)
		}
	}
	for _, question := range data.PendingCache {
		if contains(question.Tags, tag) {
			tempData.PendingCache = append(tempData.PendingCache, question)
		}
	}
	for _, question := range data.UpdatingCache {
		if contains(question.Tags, tag) {
			tempData.UpdatingCache = append(tempData.UpdatingCache, question)
		}
	}
	tempData.Qns = data.Qns
	data.CacheLock.Unlock()

	page := template.Must(template.ParseFiles("public/template.html"))
	if err := page.Execute(w, writeResponse(user, tempData, c, tag)); err != nil {
		c.Criticalf("%v", err.Error())
	}
}

// Handler to find all questions answered/being answered by the user in URL
func userHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, user stackongo.User) {
	userID := r.FormValue("id")

	page := template.Must(template.ParseFiles("public/template.html"))

	query := readUserFromDb(userID)
	userID_int, _ := strconv.Atoi(userID)

	// Create and fill in a new webData struct
	tempData := webData{}

	data.CacheLock.Lock()
	// range through the question caches golang stackongo and add if the question contains the tag
	tempData.UnansweredCache = data.UnansweredCache
	for _, question := range data.AnsweredCache {
		if data.Qns[question.Question_id].User_id == userID_int {
			tempData.AnsweredCache = append(tempData.AnsweredCache, question)
		}
	}
	for _, question := range data.PendingCache {
		if data.Qns[question.Question_id].User_id == userID_int {
			tempData.PendingCache = append(tempData.PendingCache, question)
		}
	}
	for _, question := range data.UpdatingCache {
		if data.Qns[question.Question_id].User_id == userID_int {
			tempData.UpdatingCache = append(tempData.UpdatingCache, question)
		}
	}
	tempData.Qns = data.Qns
	data.CacheLock.Unlock()

	if err := page.Execute(w, writeResponse(user, tempData, c, query.Display_name)); err != nil {
		c.Criticalf("%v", err.Error())
	}
}

//This is the main tags page
//Should display a list of tags that are logged in the database
//User can either click on a tag to view any questions containing that tag or search by a specific tag
func viewTagsHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, user stackongo.User) {
	page := template.Must(template.ParseFiles("public/viewTags.html"))
	//Read all tags and their counts from the db, and execute the page
	query := readTagsFromDb()
	//Format array of tags into another array, to be easier formatted on the page into a table
	//An array of tagData arrays of size 4
	var tagArray [][]tagData
	var tempTagArray []tagData
	i := 0
	for _, t := range query {
		tempTagArray = append(tempTagArray, t)
		i++
		if i == 4 {
			tagArray = append(tagArray, tempTagArray)
			i = 0
			//clear the temp array.
			tempTagArray = nil
		}
	}
	tagArray = append(tagArray, tempTagArray)
	if err := page.Execute(w, tagArray); err != nil {
		c.Criticalf("%v", err.Error())
	}

}

func viewUsersHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, user stackongo.User) {
	page := template.Must(template.ParseFiles("public/viewUsers.html"))
	query := readUsersFromDb()
	//uname, _ := strconv.Atoi(r.FormValue("username"))
	//query := users
	log.Println(len(users))
	if err := page.Execute(w, query); err != nil {
		c.Criticalf("%v", err.Error())
	}
}

func userPageHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, user stackongo.User) {
	page := template.Must(template.ParseFiles("public/userPage.html"))
	currentUser, _ := strconv.Atoi(r.FormValue("userId"))
	query := users[currentUser]
	if err := page.Execute(w, query); err != nil {
		c.Criticalf("%v", err.Error())
	}
}

func getUser(w http.ResponseWriter, r *http.Request, c appengine.Context) stackongo.User {
	// Collect access token from browswer cookie
	// If cookie does not exist, obtain token using code from URL and set as cookie
	// If code does not exist, redirect to login page for authorization
	cookie, err := r.Cookie("access_token")
	var token string
	if err != nil {
		code := r.URL.Query().Get("code")
		if code == "" {
			c.Infof("Returning Guest user")
			return guest
		}
		access_tokens, err := backend.ObtainAccessToken(code)
		if err != nil {
			c.Errorf(err.Error())
			return guest
		}
		c.Infof("Setting cookie: access_token")
		token = access_tokens["access_token"]
		http.SetCookie(w, &http.Cookie{Name: "access_token", Value: token})
	} else {
		token = cookie.Value
	}
	user, err := backend.AuthenticatedUser(map[string]string{}, token)
	addUser(user)
	if err != nil {
		c.Errorf(err.Error())
		return guest
	}
	return user
}

// Write a genReply struct with the inputted Question slices
// This can call readFromDb() now as a method, most of this is redunant.
func writeResponse(user stackongo.User, writeData webData, c appengine.Context, query string) genReply {
	return genReply{
		Wrapper: writeData.Wrapper, // The global wrapper
		Caches: []cacheInfo{ // Slices caches and their relevant info
			cacheInfo{
				CacheType: "unanswered",
				Questions: writeData.UnansweredCache,
				Info:      "These are questions that have not yet been answered by the Places API team",
			},
			cacheInfo{
				CacheType: "answered",
				Questions: writeData.AnsweredCache,
				Info:      "These are questions that have been answered by the Places API team",
			},
			cacheInfo{
				CacheType: "pending",
				Questions: writeData.PendingCache,
				Info:      "These are questions that are being answered by the Places API team",
			},
			cacheInfo{
				CacheType: "updating",
				Questions: writeData.UpdatingCache,
				Info:      "These are questions that will be answered in the next release",
			},
		},
		User:       user,          // Current user information
		Qns:        writeData.Qns, // Map users by questions answered
		UpdateTime: mostRecentUpdate,
		Query:      query,
	}
}

// Handler for errors
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

// Initializes userData struct
func (user userData) init(u stackongo.User, token string) {
	user.User_info = u
	user.access_token = token
	user.answeredCache = []stackongo.Question{}
	user.pendingCache = []stackongo.Question{}
	user.updatingCache = []stackongo.Question{}
}
