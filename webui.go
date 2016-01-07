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
	Wrapper   *stackongo.Questions            // Request information
	Caches    map[string][]stackongo.Question // Caches by question states
	Qns       map[int]stackongo.User          // Map of users by question ids
	Users     map[int]*userData               // Map of users by user ids
	CacheLock sync.Mutex                      // For multithreading, will use to avoid updating cache and serving cache at the same time
}

type userData struct {
	user_info     stackongo.User       // SE user info
	access_token  string               // Token to access info
	answeredCache []stackongo.Question //  answered by user
	pendingCache  []stackongo.Question // Questions being answered by user
	updatingCache []stackongo.Question // Questions that are being updated
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
		Users: make(map[int]*userData),
	}
}

const timeout = 1 * time.Minute

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

	// Initialising stackongo session
	backend.NewSession()

	// defining the duration to pull questions from
	day := 24 * time.Hour
	week := 7 * day
	toDate := time.Now()
	fromDate := toDate.Add(-1*week + day)

	// goroutine to collect the questions from SO and add them to the database
	go func(fromDate time.Time, toDate time.Time, db *sql.DB) {
		// Iterate over ([SPECIFIED DURATION])
		for i := 0; i < 0; i++ {
			// Collect new questions from SO
			questions, err := backend.GetNewQns(fromDate, toDate)
			if err != nil {
				log.Printf("Error getting new questions: %v", err.Error())
				continue
			}

			// Add new questions to database
			if err = backend.AddQuestions(db, questions); err != nil {
				log.Printf("Error updating database: %v", err.Error())
				continue
			}

			// adjust the date to the time until the next pull
			toDate = time.Now()
			fromDate = toDate.Add(-1*week + day)
		}
	}(fromDate, toDate, db)

	log.Println("Initial cache download")
	refreshLocalCache()

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

// Handler for checking if the database has been updated
func updateHandler(w http.ResponseWriter, r *http.Request) {

	time, _ := strconv.Atoi(r.FormValue("time"))
	page, _ := template.New("updatePage").Parse("{{$.CacheUpdated}}")
	if err := page.Execute(w, genReply{UpdateTime: int32(time)}); err != nil {
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

	page := template.Must(template.ParseFiles("public/template.html"))
	if err := page.Execute(w, writeResponse(user, data, c, "")); err != nil {
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
	if err := page.Execute(w, writeResponse(user, tempData, c, tag)); err != nil {
		c.Criticalf("%v", err.Error())
	}
}

// Handler to find all questions answered/being answered by the user in URL
func userHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, user stackongo.User) {
	userID, _ := strconv.Atoi(r.FormValue("id"))
	query := data.Users[userID]

	// Create and fill in a new webData struct
	tempData := webData{}

	data.CacheLock.Lock()
	// range through the question caches golang stackongo and add if the question contains the tag
	tempData.Caches["unanswered"] = data.Caches["unanswered"]
	for cacheType, cache := range data.Caches {
		if cacheType != "unanswered" {
			for _, question := range cache {
				if data.Qns[question.Question_id].User_id == userID {
					tempData.Caches[cacheType] = append(tempData.Caches[cacheType], question)
				}
			}
		}
	}
	tempData.Qns = data.Qns
	data.CacheLock.Unlock()

	page := template.Must(template.ParseFiles("public/template.html"))
	if err := page.Execute(w, writeResponse(user, tempData, c, query.user_info.Display_name)); err != nil {
		c.Criticalf("%v", err.Error())
	}
}

func viewTagsHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, user stackongo.User) {
	page := template.Must(template.ParseFiles("/public/viewTags.html"))
	tempData := webData{}
	if err := page.Execute(w, writeResponse(user, tempData, c, "thing")); err != nil {
		c.Criticalf("%v", err.Error())
	}

}

func viewUsersHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, user stackongo.User) {

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
func writeResponse(user stackongo.User, writeData webData, c appengine.Context, query string) genReply {
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
func newUser(u stackongo.User, token string) *userData {
	return &userData{
		user_info:    u,
		access_token: token,
	}
}
