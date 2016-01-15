/*

 */

package webui

import (
	"backend"
	"database/sql"
	"html/template"
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
	UpdateTime int64
	Query      []string				  // String array holding query and query type (tag vs user)
}

type queryReply struct {
	User stackongo.User
	Data interface{}
}

// Info on the various caches
type cacheInfo struct {
	CacheType string               // "unanswered"/"answered"/"pending"/"updating"
	Questions []stackongo.Question // list of questions
	Info      string               // blurb about the cache
}

// Data struct with SO information, caches, user information
type webData struct {
	Wrapper   *stackongo.Questions            // Request information
	Caches    map[string][]stackongo.Question // Caches by question states
	Qns       map[int]stackongo.User          // Map of users by question ids
	Users     map[int]userData                // Map of users by user ids
	CacheLock sync.Mutex                      // For multithreading, will use to avoid updating cache and serving cache at the same time
}

type userData struct {
	User_info    stackongo.User                  // SE user info
	Access_token string                          // Token to access info
	Caches       map[string][]stackongo.Question // questions modified by user sorted into cacheTypes
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
		Caches: map[string][]stackongo.Question{
			"unanswered": []stackongo.Question{},
			"answered":   []stackongo.Question{},
			"pending":    []stackongo.Question{},
			"updating":   []stackongo.Question{},
		},
		Qns:   make(map[int]stackongo.User),
		Users: make(map[int]userData),
	}
}

const timeout = 6 * time.Hour

// Global variable with cache info
var data = newWebData()

// Standard guest user
var guest = stackongo.User{
	Display_name: "Guest",
}

// Pointer to database connection to communicate with Cloud SQL
var db = backend.SqlInit()

//Stores the last time the database was read into the cache
//This is then checked against the update time of the database and determine whether the cache should be updated
var mostRecentUpdate int64
var recentChangedQns []string

func (r genReply) CacheUpdated() bool {
	return mostRecentUpdate > r.UpdateTime
}
func (r genReply) Timestamp(timeUnix int64) time.Time {
	return time.Unix(timeUnix, 0)
}

//The app engine will run its own main function and imports this code as a package
//So no main needs to be defined
//All routes go in to init
func init() {

	recentChangedQns = []string{}

	// Initialising stackongo session
	backend.NewSession()

	// goroutine to collect the questions from SO and add them to the database
	go func(db *sql.DB) {
		// Iterate over ([SPECIFIED DURATION])
		for _ = range time.NewTicker(timeout).C {
			toDate := time.Now()
			fromDate := toDate.Add(-1 * timeout)
			// Collect new questions from SO
			log.Println("Getting new questions")
			questions, err := backend.GetNewQns(fromDate, toDate)
			if err != nil {
				log.Printf("Error getting new questions: %v", err.Error())
				continue
			}

			log.Println("Adding questions to db")
			// Add new questions to database
			if err = backend.AddQuestions(db, questions); err != nil {
				log.Printf("Error updating database: %v", err.Error())
				continue
			}
		}
	}(db)

	log.Println("Initial cache download")
	initCacheDownload()

	// goroutine to update local cache if there has been any change to database
	count := 1
	go func(count int) {
		for {
			if checkDBUpdateTime("questions", mostRecentUpdate) {
				log.Printf("Refreshing cache %v", count)
				refreshLocalCache()
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
	http.HandleFunc("/search", handler)
}

// Handler for authorizing user
func authHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Redirecting to SO login")
	auth_url := backend.AuthURL()
	header := w.Header()
	header.Add("Location", auth_url)
	w.WriteHeader(302)
}

// Handler for checking if the database has been updated
func updateHandler(w http.ResponseWriter, r *http.Request) {

	time, _ := strconv.ParseInt(r.FormValue("time"), 10, 64)
	pageText := "Updated: {{$.CacheUpdated}}\n"
	for _, question := range recentChangedQns {
		pageText += question + "\n"
	}
	page, _ := template.New("updatePage").Parse(pageText)
	if err := page.Execute(w, genReply{UpdateTime: time}); err != nil {
		log.Printf("%v", err.Error())
	}
}

// Handler for main information to be read and written from
func handler(w http.ResponseWriter, r *http.Request) {
	// Create a new appengine context for logging purposes
	c := appengine.NewContext(r)

	// set the appengine transport using the http request
	backend.SetTransport(r)

	// get the current user
	user := getUser(w, r, c)

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

	if r.URL.Path == "/search" {
		searchHandler(w, r, c, user)
		return
	}

	page := template.Must(template.ParseFiles("public/template.html"))
	pageQuery := []string {
		"",
		"",
	}
	// WriteResponse creates a new response with the various caches
	if err := page.Execute(w, writeResponse(user, data, c, pageQuery)); err != nil {
		c.Criticalf("%v", err.Error())
	}
}

//Handler for keywords, tags, users in the search box
/* 
	SQL Fields: Title, Body, Link, Tags, Users.
*/
func searchHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, user stackongo.User) {
	//Collect query
	search := r.FormValue("search")
	//Convert to string to check ID against questions	
	i, err := strconv.Atoi(search)
	searchType := ""
	if (err != nil) {
		i = 0
		searchType = "URL"
	} else {
		searchType = "ID"
	}
	tempData := newWebData()

	//data.CacheLock.Lock()
	//Range through questions, checking URL and question ID against search term
	for cacheType, cache := range data.Caches {
		for _, question := range cache {
			if (question.Question_id == i || question.Link == search) {
				tempData.Caches[cacheType] = append(tempData.Caches[cacheType], question)
			}
		}
	}

	tempData.Qns = data.Qns
	data.CacheLock.Lock()

	page := template.Must(template.ParseFiles("public/template.html"))

	var pageQuery = []string {
		searchType,
		search,
	}
	if err := page.Execute(w, writeResponse(user, tempData, c, pageQuery)); err != nil {
		c.Criticalf("%v", err.Error())
	}


}

// Handler to find all questions with specific tags
func tagHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, user stackongo.User) {
	// Collect query
	tag := r.FormValue("tagSearch")
	// Create and fill in a new webData struct
	tempData := newWebData()

	data.CacheLock.Lock()
	// range through the question caches golang stackongoand add if the question contains the tag
	for cacheType, cache := range data.Caches {
		for _, question := range cache {
			if contains(question.Tags, tag) {
				tempData.Caches[cacheType] = append(tempData.Caches[cacheType], question)
			}
		}
	}
	tempData.Qns = data.Qns
	data.CacheLock.Unlock()

	page := template.Must(template.ParseFiles("public/template.html"))
	var tagQuery = []string {
		"tag", 
		tag,
	}
	if err := page.Execute(w, writeResponse(user, tempData, c, tagQuery)); err != nil {
		c.Criticalf("%v", err.Error())
	}
}

// Handler to find all questions answered/being answered by the user in URL
func userHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, user stackongo.User) {
	userID, _ := strconv.Atoi(r.FormValue("id"))
	query := userData{}

	// Create and fill in a new webData struct
	tempData := newWebData()

	data.CacheLock.Lock()
	// range through the question caches golang stackongo and add if the question contains the tag
	tempData.Caches["unanswered"] = data.Caches["unanswered"]
	if userQuery, ok := data.Users[userID]; ok {
		query = userQuery
		for cacheType, cache := range data.Users[userID].Caches {
			if cacheType != "unanswered" {
				tempData.Caches[cacheType] = cache
			}
		}
		tempData.Qns = data.Qns
	}
	data.CacheLock.Unlock()
	page := template.Must(template.ParseFiles("public/template.html"))
	
	var userQuery = []string {
		"user",
		query.User_info.Display_name,
	}
	if err := page.Execute(w, writeResponse(user, tempData, c, userQuery)); err != nil {
		c.Criticalf("%v", err.Error())
	}
}

//This is the main tags page
//Should display a list of tags that are logged in the database
//User can either click on a tag to view any questions containing that tag or search by a specific tag
func viewTagsHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, user stackongo.User) {
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
	page := template.Must(template.ParseFiles("public/viewTags.html"))
	if err := page.Execute(w, queryReply{user, tagArray}); err != nil {
		c.Criticalf("%v", err.Error())
	}

}

func viewUsersHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, user stackongo.User) {
	page := template.Must(template.ParseFiles("public/viewUsers.html"))
	query := data.Users
	if err := page.Execute(w, queryReply{user, query}); err != nil {
		c.Criticalf("%v", err.Error())
	}
}

func userPageHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, user stackongo.User) {
	page := template.Must(template.ParseFiles("public/userPage.html"))
	usr, _ := strconv.Atoi(r.FormValue("userId"))
	currentUser := data.Users[usr]
	query := userData{User_info: currentUser.User_info}

	var n int
	query.Caches = make(map[string][]stackongo.Question)

	n = Min(3, len(currentUser.Caches["unanswered"]))
	if n > 0 {
		query.Caches["answered"] = currentUser.Caches["answered"][0:n]
	}
	n = Min(3, len(currentUser.Caches["pending"]))
	if n > 0 {
		query.Caches["pending"] = currentUser.Caches["pending"][0:n]
	}

	n = Min(3, len(currentUser.Caches["updating"]))
	if n > 0 {
		query.Caches["updating"] = currentUser.Caches["updating"][0:n]
	}
	if err := page.Execute(w, queryReply{user, query}); err != nil {
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
	if err != nil {
		c.Errorf(err.Error())
		return guest
	}
	data.CacheLock.Lock()
	if _, ok := data.Users[user.User_id]; !ok {
		data.Users[user.User_id] = newUser(user, token)
		addUserToDB(user)
	}
	data.CacheLock.Unlock()
	return user
}

// Write a genReply struct with the inputted Question slices
// This can call readFromDb() now as a method, most of this is redundant.
func writeResponse(user stackongo.User, writeData webData, c appengine.Context, query []string) genReply {
	return genReply{
		Wrapper: writeData.Wrapper, // The global wrapper
		Caches: []cacheInfo{ // Slices caches and their relevant info
			cacheInfo{
				CacheType: "unanswered",
				Questions: writeData.Caches["unanswered"],
				Info:      "These are questions that have not yet been answered by the Places API team",
			},
			cacheInfo{
				CacheType: "answered",
				Questions: writeData.Caches["answered"],
				Info:      "These are questions that have been answered by the Places API team",
			},
			cacheInfo{
				CacheType: "pending",
				Questions: writeData.Caches["pending"],
				Info:      "These are questions that are being answered by the Places API team",
			},
			cacheInfo{
				CacheType: "updating",
				Questions: writeData.Caches["updating"],
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
func newUser(u stackongo.User, token string) userData {
	return userData{
		User_info:    u,
		Access_token: token,
		Caches: map[string][]stackongo.Question{
			"answered": []stackongo.Question{},
			"pending":  []stackongo.Question{},
			"updating": []stackongo.Question{},
		},
	}
}

func Min(x int, y int) int {
	if x < y {
		return x
	} else {
		return y
	}
}
