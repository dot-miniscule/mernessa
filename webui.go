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
	"sort"
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
	Wrapper *stackongo.Questions   // Information about the query
	Caches  []cacheInfo            // Slice of the 4 caches (Unanswered, Answered, Pending, Updating)
	User    stackongo.User         // Information on the current user
	Qns     map[int]stackongo.User // Map of users by question ids
}

// Info on the various caches
type cacheInfo struct {
	CacheType string               // "unanswered"/"answered"/"pending"/"updating"
	Questions []stackongo.Question // list of questions
	Info      string               // blurb about the cache
}

type webData struct {
	lastUpdateTime  int64                // Time the cache was last updated in Unix
	wrapper         *stackongo.Questions // Request information
	unansweredCache []stackongo.Question // Unanswered questions
	answeredCache   []stackongo.Question // Answered questions
	pendingCache    []stackongo.Question // Pending questions
	updatingCache   []stackongo.Question // Updating questions
	cacheLock       sync.Mutex           // For multithreading, will use to avoid updating cache and serving cache at the same time
}

type userData struct {
	user_info     stackongo.User       // SE user info
	access_token  string               // Token to access info
	answeredCache []stackongo.Question // Questions answered by user
	pendingCache  []stackongo.Question // Questions being answered by user
	updatingCache []stackongo.Question // Questions that are being updated
}

// Global variable with cache info
var pageData = webData{}
var data = webData{}

// Map of users by user ids
var users = make(map[int]*userData)

// Map relating question ids to users
var qns = make(map[int]stackongo.User)

// Standard guest user
var guest = stackongo.User{
	Display_name: "guest",
}

// Pointer to database connection to communicate with Cloud SQL
var db *sql.DB

//Stores the last time the database was read into the cache
//This is then checked against the update time of the database and determine whether the cache should be updated
var mostRecentUpdate int32

// Functions for template to recieve data from maps
func (r genReply) GetUserID(id int) int {
	return r.Qns[id].User_id
}
func (r genReply) GetUserName(id int) string {
	return r.Qns[id].Display_name
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
	//Initalize db
	db = backend.SqlInit()

	//Read questions from Stack wrapper
	pageData.wrapper = new(stackongo.Questions) // Create a new wrapper
	if err := json.Unmarshal(input, pageData.wrapper); err != nil {
		fmt.Println(err.Error())
		return
	}
	//Comment Out the next line to avoid ridiculous loading times while in development phase
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
	//Check if the database needs to be updated again based on the last refresh time.
	if checkDBUpdateTime("questions") == true {
		data = readFromDb()
		mostRecentUpdate = int32(time.Now().Unix())
	}
	http.HandleFunc("/login", authHandler)
	http.HandleFunc("/", handler)
	http.HandleFunc("/tag", handler)
	http.HandleFunc("/user", handler)
}

func readFromDb() webData {
	//Reading from database
	log.Println("Refreshing database read")
	tempData := webData{}
	var (
		url   string
		title string
		id    int
		state string
	)
	//Select all questions in the database and read into a new data object
	rows, err := db.Query("select * from questions")
	if err != nil {
		log.Fatal("query failed:\t", err)
	}

	defer rows.Close()
	//Iterate through each row and add to the correct cache
	for rows.Next() {
		err := rows.Scan(&id, &title, &url, &state)
		currentQ := stackongo.Question{
			Question_id: id,
			Title:       title,
			Link:        url,
		}
		if err != nil {
			log.Fatal("query failed:\t", err)
		}

		//Switch on the state as read from the database to ensure question is added to correct cace
		switch state {
		case "unanswered":
			tempData.unansweredCache = append(tempData.unansweredCache, currentQ)
		case "answered":
			tempData.answeredCache = append(tempData.answeredCache, currentQ)
		case "pending":
			tempData.pendingCache = append(tempData.pendingCache, currentQ)
		case "updating":
			tempData.updatingCache = append(tempData.updatingCache, currentQ)

		}
	}
	mostRecentUpdate = int32(time.Now().Unix())
	return tempData
}

/* Function to check if the DB has been updated since we last queried it
Returns true if our cache needs to be refreshed
False if is all g */
func checkDBUpdateTime(tableName string) bool {
	var (
		id           int
		table_name   string
		last_updated int32
	)
	rows, err := db.Query("SELECT last_updated FROM update_times WHERE table_name ='?'")
	if err != nil {
		log.Fatal("Query failed:\t", err)
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&id, &table_name, &last_updated)
		if err != nil {
			log.Fatal(err)
		}
	}
	if err != nil {
		log.Fatal(err)
	}
	if last_updated > mostRecentUpdate {
		return false
	} else {
		return true
	}
}

//A crude way to find out if the working cache needs to be refreshed from the database.
//Stores the current Unix time in update_times table on Cloud SQL */
func updateTableTimes(tableName string) {
	stmts, err := db.Prepare("UPDATE update_times SET last_updated=? WHERE table_name=?")
	if err != nil {
		log.Println("Prepare failed:\t", err)
	}

	_, err = stmts.Exec(int32(time.Now().Unix()), tableName)
	if err != nil {
		log.Println("Could not update time for", tableName+":\t", err)
	} else {
		log.Println("Update time for", tableName, "successfully updated!")
	}
}

// Handler for authorizing user
func authHandler(w http.ResponseWriter, r *http.Request) {
	auth_url := backend.AuthURL()
	header := w.Header()
	header.Add("Location", auth_url)
	w.WriteHeader(302)
}

// Handler for main information to be read and written from
func handler(w http.ResponseWriter, r *http.Request) {
	// Create a new appengine context for logging purposes
	c := appengine.NewContext(r)

	backend.SetTransport(r)
	_ = backend.NewSession(r)

	// Collect access token from browswer cookie
	// If cookie does not exist, obtain token using code from URL and set as cookie
	// If code does not exist, redirect to login page for authorization
	token, err := r.Cookie("access_token")
	var access_tokens map[string]string
	if err != nil {
		code := r.URL.Query().Get("code")
		if code != "" {
			c.Infof("Getting new user code")
			handler(w, r)
			return
		}
		access_tokens, err = backend.ObtainAccessToken(code)
		if err == nil {
			c.Infof("Setting cookie: access_token")
			http.SetCookie(w, &http.Cookie{Name: "access_token", Value: access_tokens["access_token"]})
		} else {
			c.Errorf(err.Error())
			errorHandler(w, r, 0, err.Error())
			return
		}
	}

	user, err := backend.AuthenticatedUser(map[string]string{}, token.Value)
	if err != nil {
		c.Errorf(err.Error())
		errorHandler(w, r, 0, err.Error())
		return
	}

	// update the new cache on submit
	cookie, _ := r.Cookie("submitting")
	if cookie != nil {
		if cookie.Value == "true" {
			err = updatingCache_User(r, c, user)
			if err != nil {
				c.Errorf(err.Error())
			}
			http.SetCookie(w, &http.Cookie{Name: "submitting", Value: ""})
		}
	}

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

	page := template.Must(template.ParseFiles("public/template.html"))
	// WriteResponse creates a new response with the various caches
	if err := page.Execute(w, writeResponse(user, c)); err != nil {
		c.Criticalf("%v", err.Error())
	}

}

// Handler to find all questions with specific tags
func tagHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, user stackongo.User) {
	/*
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
	*/
	page := template.Must(template.ParseFiles("public/template.html"))
	if err := page.Execute(w, writeResponse(user, c)); err != nil {
		c.Criticalf("%v", err.Error())
	}
}

// Handler to find all questions answered/being answered by the user in URL
func userHandler(w http.ResponseWriter, r *http.Request, c appengine.Context, user stackongo.User) {
	userID, _ := strconv.Atoi(r.FormValue("id"))

	page := template.Must(template.ParseFiles("public/template.html"))

	if _, ok := users[userID]; !ok {
		page.Execute(w, writeResponse(user, c))
		return
	}
	if err := page.Execute(w, writeResponse(user, c)); err != nil {
		c.Criticalf("%v", err.Error())
	}
}

// Write a genReply struct with the inputted Question slices
// This can call readFromDb() now as a method, most of this is redunant.
func writeResponse(user stackongo.User, c appengine.Context) genReply {
	var data = webData{}
	//Reading from database
	var (
		url   string
		title string
		id    int
		state string
	)
	rows, err := db.Query("select * from questions")
	if err != nil {
		log.Fatal("query failed:\t", err)
	}
	updateTableTimes("questions")
	defer func() {
		c.Infof("Closing rows: WriteResponse")
		rows.Close()
	}()

	for rows.Next() {
		err := rows.Scan(&id, &title, &url, &state)
		if err != nil {
			c.Criticalf(err.Error())
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
			data.answeredCache = append(data.answeredCache, currentQ)
		case "pending":
			data.pendingCache = append(data.pendingCache, currentQ)
		case "updating":
			data.updatingCache = append(data.updatingCache, currentQ)
		}
	}
	return genReply{
		Wrapper: pageData.wrapper, // The global wrapper
		Caches: []cacheInfo{ // Slices caches and their relevant info
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
		User: user, // Current user information
		Qns:  qns,  // Map users by questions answered
	}
}

// updating the caches based on input from the appi
// Vanessa TODO: Can this be migrated to the readFromDb() method? -- Meredith
func updatingCache_User(r *http.Request, c appengine.Context, user stackongo.User) error {
	c.Infof("updating cache")
	if checkDBUpdateTime("questions") /* time on sql db is later than lastUpdatedTime */ {
		data = readFromDb()
		mostRecentUpdate = int32(time.Now().Unix())
	}

	// required to collect post form data
	r.ParseForm()

	// If the user is not in the database, add a new entry
	if _, ok := users[user.User_id]; !ok {
		users[user.User_id] = &userData{}
		users[user.User_id].init(user, "")
	}

	//tempData := webData{}

	// Collect the submitted form info based on the name of the form
	// Check each cache against the form data
	// Read from db based on each question id (primary key) to retrieve and update the state
	var (
		url   string
		title string
		id    int
		numQ  int
	)
	type rowData struct {
		state    string
		question stackongo.Question
	}
	var newState string

	rows, err := db.Query("select * from questions")
	if err != nil {
		c.Errorf("query failed:\t%v", err)
	}
	defer func() {
		c.Infof("closing rows: updating")
		rows.Close()
	}()

	//	channel := make(chan rowData)

	for rows.Next() {
		numQ++
		//go func(rows *sql.Rows) {
		row := rowData{}
		err := rows.Scan(&id, &title, &url, &row.state)
		if err != nil {
			c.Errorf("rows.Scan: %v", err.Error())
		}
		question := stackongo.Question{
			Question_id: id,
			Title:       title,
			Link:        url,
		}
		row.question = question
		/*	channel <- row
				}(rows)
			}
			if err = rows.Err(); err != nil {
				c.Errorf(err.Error())
			}

			for i := 0; i < numQ; i++ {
				row := <-channel*/
		name := row.state + "_" + strconv.Itoa(id)
		form_input := r.PostFormValue(name)
		switch form_input {
		case "unanswered":
			newState = "unanswered"
		case "answered":
			users[user.User_id].answeredCache = append(users[user.User_id].answeredCache, row.question)
			newState = "answered"
		case "pending":
			users[user.User_id].pendingCache = append(users[user.User_id].pendingCache, row.question)
			newState = "pending"
		case "updating":
			users[user.User_id].updatingCache = append(users[user.User_id].updatingCache, row.question)
			newState = "updating"
		case "no_change":
			newState = row.state
		}

		// If the question is now unanswered, delete question from map
		if form_input == "unanswered" {
			qns[row.question.Question_id] = stackongo.User{}
			delete(qns, row.question.Question_id)

			// Else remove question from original editor's cache and map user to question
		} else if form_input != "no_change" {

			editor := qns[row.question.Question_id]
			if editor.User_id != 0 {
				for i, q := range users[editor.User_id].answeredCache {
					if row.question.Question_id == q.Question_id {
						users[editor.User_id].answeredCache = append(users[editor.User_id].answeredCache[:i], users[editor.User_id].answeredCache[i+1:]...)
						break
					}
				}
			}

			qns[row.question.Question_id] = user
		}

		if newState != row.state {
			stmts, err := db.Prepare("UPDATE questions SET state=? where question_id=?")
			if err != nil {
				c.Errorf("%v", err.Error())
			}
			_, err = stmts.Exec(newState, row.question.Question_id)
			if err != nil {
				c.Errorf("Update query failed:\t%v", err.Error())
			}
		}
	}
	//Update the table on SQL keeping track of table modifications
	updateTableTimes("questions")

	/* old updating method
	for _, question := range data.answeredCache {
		newState = "answered"
		name := "answered_" + strconv.Itoa(question.Question_id)
		form_input := r.PostFormValue(name)
		switch form_input {
		case "unanswered":
			tempData.unansweredCache = append(tempData.unansweredCache, question)
			newState = "unanswered"
		case "answered":
			tempData.answeredCache = append(tempData.answeredCache, question)
			users[user.User_id].answeredCache = append(users[user.User_id].answeredCache, question)
			newState = "answered"
		case "pending":
			tempData.pendingCache = append(tempData.pendingCache, question)
			users[user.User_id].pendingCache = append(users[user.User_id].pendingCache, question)
			newState = "pending"
		case "updating":
			tempData.updatingCache = append(tempData.updatingCache, question)
			users[user.User_id].updatingCache = append(users[user.User_id].updatingCache, question)
			newState = "updating"
		case "no_change":
			tempData.answeredCache = append(tempData.answeredCache, question)
		}

		// If the question is now unanswered, delete question from map
		if form_input == "unanswered" {
			qns[question.Question_id] = stackongo.User{}
			delete(qns, question.Question_id)

			// Else remove question from original editor's cache and map user to question
		} else if form_input != "no_change" {

			editor := qns[question.Question_id]
			for i, q := range users[editor.User_id].answeredCache {
				if question.Question_id == q.Question_id {
					users[editor.User_id].answeredCache = append(users[editor.User_id].answeredCache[:i], users[editor.User_id].answeredCache[i+1:]...)
					break
				}
			}

			qns[question.Question_id] = user
		}

		if newState != "answered" {
			stmts, err := db.Prepare("UPDATE questions SET state=? where question_id=?")
			if err != nil {
				log.Println(err)
			}
			_, err = stmts.Exec(newState, question.Question_id)
			if err != nil {
				log.Println("Update query failed:\t", err)
			}
		}
	}
	*/

	// sort user caches by creation date
	sort.Stable(byCreationDate(users[user.User_id].answeredCache))
	sort.Stable(byCreationDate(users[user.User_id].pendingCache))
	sort.Stable(byCreationDate(users[user.User_id].updatingCache))

	return nil
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
	user.user_info = u
	user.access_token = token
	user.answeredCache = []stackongo.Question{}
	user.pendingCache = []stackongo.Question{}
	user.updatingCache = []stackongo.Question{}
}
