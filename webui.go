/*

 */

package webui

import (
	"backend"
	"database/sql"
	"encoding/json"
	"html/template"
	"io/ioutil"
	"os"

	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/laktek/Stack-on-Go/stackongo"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
)

// Functions for sorting
type byCreationDate []stackongo.Question
type ByDisplayName []userData

func (a byCreationDate) Len() int           { return len(a) }
func (a byCreationDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byCreationDate) Less(i, j int) bool { return a[i].Creation_date > a[j].Creation_date }

func (a ByDisplayName) Len() int      { return len(a) }
func (a ByDisplayName) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByDisplayName) Less(i, j int) bool {
	return a[i].User_info.Display_name < a[j].User_info.Display_name
}

// Reply to send to main template
type genReply struct {
	Wrapper    *stackongo.Questions   // Information about the query
	Caches     []cacheInfo            // Slice of the 4 caches (Unanswered, Answered, Pending, Updating)
	User       stackongo.User         // Information on the current user
	Qns        map[int]stackongo.User // Map of users by question ids
	UpdateTime int64
	Query      []string // String array holding query and query type (tag vs user)
}

// Generic reply to send to other templates
type queryReply struct {
	User       stackongo.User
	UpdateTime int64
	Page       int
	LastPage   int
	Data       interface{}
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

// User information and the user's caches
type userData struct {
	User_info stackongo.User                  // SE user info
	Caches    map[string][]stackongo.Question // Questions modified by user sorted into cacheTypes
}

// Information on tags
type tagData struct {
	Tag   string //The actual tag, hyphenated string
	Count int    //The number of questions with that tag in the db
}

// Simplified user struct
type userInfo struct {
	ID   int
	Name string
	Pic  string
	Link string
}

// Creates an initialised webData struct
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

const timeout = 6 * time.Hour // Time to wait between querying new SE questions

// Standard guest user
var guest = stackongo.User{
	Display_name: "Guest",
}

// Pointer to database connection to communicate with Cloud SQL
var db *sql.DB
var DB_STRING = ""

//Stores the last time the database was read into the cache
//This is then checked against the update time of the database and determine whether the cache should be updated
var lastPull = time.Now().Add(-1 * time.Hour * 24 * 7).Unix()
var recentChangedQns = []string{} // Array of the most recently changed questions
var mostRecentUpdate int64        // Time of most recent update

/* --------- Template functions ------------ */
// Returns timeUnix as a formatted string
func (r genReply) Timestamp(timeUnix int64) string {
	est, _ := time.LoadLocation("Australia/Sydney")
	timeFormat := "Jan 2 at 15:04 2006"
	return time.Unix(timeUnix, 0).In(est).Format(timeFormat)
}

// Returns current page + num
func (r queryReply) PagePlus(num int) int {
	return r.Page + num
}

//The app engine will run its own main function and imports this code as a package
//So no main needs to be defined
//All routes go in to init
func init() {
	recentChangedQns = []string{}
	lastPull = time.Now().Add(-1 * time.Hour * 24 * 7).Unix()

	// Initialising stackongo session
	backend.NewSession()

	// Handlers for pages
	http.HandleFunc("/login", authHandler)
	http.HandleFunc("/", handler)
	http.HandleFunc("/tag", handler)
	http.HandleFunc("/user", handler)
	http.HandleFunc("/viewTags", handler)
	http.HandleFunc("/viewUsers", handler)
	http.HandleFunc("/userPage", handler)
	http.HandleFunc("/dbUpdated", updateHandler)
	http.HandleFunc("/search", handler)
	http.HandleFunc("/addQuestion", handler)
	http.HandleFunc("/pullNewQn", handler)
}

func checkForDBConnection() bool {
	return !(DB_STRING == "")
}

func connectToDB(ctx context.Context) {
	version := appengine.VersionID(ctx)
	versionID := strings.Split(version, ".")
	log.Infof(ctx, "Current serving app version: %s", versionID[0])
	if versionID[0] == "live" {
		DB_STRING = os.Getenv("LIVE_DB")
	} else {
		DB_STRING = os.Getenv("TEST_DB")
	}
	log.Infof(ctx, "connecting to DB with string %s", DB_STRING)
	db = backend.SqlInit(DB_STRING)
}

// Handler for authorizing user
// Redirects user to a url for authentication
// Once authenticated, returns to the home page with a code which we use to get the current user
func authHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	log.Infof(ctx, "Redirecting to SO login")
	auth_url := backend.AuthURL(r.Header["Referer"][0])
	header := w.Header()
	header.Add("Location", auth_url)
	w.WriteHeader(302)
}

// Handler for checking if the database has been updated
// Writes a JSON object to the page
// ie. {"Updated": true, "Questions: ["Title1", "Title2"]}
func updateHandler(w http.ResponseWriter, r *http.Request) {

	time, _ := strconv.ParseInt(r.FormValue("time"), 10, 64)

	// Writing the page to JSON format
	pageText := "{\"Updated\": " + strconv.FormatBool(mostRecentUpdate > time) + ","
	pageText += "\"Questions\": ["
	for _, question := range recentChangedQns {
		pageText += "\"" + question + "\","
	}
	pageText = strings.TrimSuffix(pageText, ",")
	pageText += "]}"

	// Write text into response
	w.Write([]byte(pageText))
}

// Handler for main information to be read and written from.
// Does following functions in order:
//   Refreshes sql db for changes to questions in SE API.
//   Updates local cache if there's been changes to the db.
//   Gets the current user to send in response.
//   If a form has been submitted, the local cache and db gets updated with new values
//   Finds the current subpage and redirects to the relevant handler
func handler(w http.ResponseWriter, r *http.Request) {

	// Set context for logging
	ctx := appengine.NewContext(r)

	if (checkForDBConnection() == false) {
		connectToDB(ctx)
	}

	backend.SetTransport(ctx)
	if strings.HasPrefix(r.URL.Path, "/pullNewQn") {
		newQnHandler(w, r, ctx)
		return
	}

	// Pull any new questions added to StackOverflow
	lastPull = updateDB(db, ctx, lastPull)

	// Get the current user
	user := getUser(w, r, ctx)

	// Collect page number
	pageNum, _ := strconv.Atoi(r.FormValue("page"))
	if pageNum == 0 {
		pageNum = 1
	}

	// Update the new cache on submit if submitting cookie is set
	cookie, _ := r.Cookie("submitting")
	if cookie != nil && cookie.Value == "true" {
		// Update the cache based on the form values sent in the request
		updateTime, err := updatingCache_User(ctx, r, user)
		if err != nil {
			log.Errorf(ctx, "Error updating cache: %v", err.Error())
		} else {
			mostRecentUpdate = updateTime
		}

		// Removing the cookie
		http.SetCookie(w, &http.Cookie{Name: "submitting", Value: ""})
	}

	// Send to valid subpages
	// else errorHandler
	if strings.HasPrefix(r.URL.Path, "/?") || strings.HasPrefix(r.URL.Path, "/home") || r.URL.Path == "/" {
		// Get data to send to page
		data, updateTime, err := readFromDb(ctx, "")
		if err != nil {
			log.Errorf(ctx, "Error reading from db: %v", err.Error())
		} else {
			mostRecentUpdate = updateTime
		}

		// Parse the html template to serve to the page
		page := template.Must(template.ParseFiles("public/template.html"))
		pageQuery := []string{
			"",
			"",
		}

		// WriteResponse creates a new response with the various caches
		if err := page.Execute(w, writeResponse(user, data, pageNum, pageQuery)); err != nil {
			log.Errorf(ctx, "%v", err.Error())
		}
	} else if strings.HasPrefix(r.URL.Path, "/tag") && r.FormValue("tagSearch") != "" {
		tagHandler(w, r, ctx, pageNum, user)
	} else if strings.HasPrefix(r.URL.Path, "/user") {
		userHandler(w, r, ctx, pageNum, user)
	} else if strings.HasPrefix(r.URL.Path, "/viewTags") {
		viewTagsHandler(w, r, ctx, pageNum, user)
	} else if strings.HasPrefix(r.URL.Path, "/viewUsers") {
		viewUsersHandler(w, r, ctx, pageNum, user)
		/*} else if strings.HasPrefix(r.URL.Path, "/userPage") {
		userPageHandler(w, r, ctx, data, pageNum, user)*/
	} else if strings.HasPrefix(r.URL.Path, "/search") {
		searchHandler(w, r, ctx, pageNum, user)
	} else if strings.HasPrefix(r.URL.Path, "/addQuestion") {
		addQuestionHandler(w, r, ctx, pageNum, user)
	} else if strings.HasPrefix(r.URL.Path, "/addNewQuestion") {
		addNewQuestionToDatabaseHandler(w, r, ctx)
	} else {
		errorHandler(w, r, ctx, http.StatusNotFound, "")
	}
}

// Handler for adding new question page
func addQuestionHandler(w http.ResponseWriter, r *http.Request, ctx context.Context,
	pageNum int, user stackongo.User) {
	page := template.Must(template.ParseFiles("public/addQuestion.html"))
	if err := page.Execute(w, queryReply{user, mostRecentUpdate, pageNum, 0, nil}); err != nil {
		log.Warningf(ctx, "%v", err.Error())
	}
}

// Handler for pulling questions from Stack Overflow manually, based on a given ID
// Request is parsed to find the supplied ID
// A check is completed to see if the question is already in the system
// If so, it retrieves that question, and returns it to be viewed, along with a message
// Makes a new backend request to retrieve new questions
// Parses the returned data into a new page, which can be inserted into the template.
func newQnHandler(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	id, _ := strconv.Atoi(r.FormValue("id"))

	res, err := backend.CheckForExistingQuestion(db, id)
	if err != nil {
		log.Infof(ctx, "QUERY FAILED, %v", err)
	}

	if res == 1 {

		existingQn := backend.PullQnByID(db, ctx, id)
		if err != nil {
			log.Warningf(ctx, err.Error())
		}
		w.Write(existingQn)
	} else {

		intArray := []int{id}
		questions, err := backend.GetQuestions(ctx, intArray)
		if err != nil {
			log.Warningf(ctx, err.Error())
		} else {
			questions.Items[0].Body = backend.StripTags(questions.Items[0].Body)
			qnJson, err := json.Marshal(questions.Items[0])
			if err != nil {
				log.Warningf(ctx, err.Error())
			}
			w.Write(qnJson)
		}
	}
}

// Handler for adding a new question to the database upon submission
// It is returned as a stringified JSON object in the request body
// The string is unmarshalled into a stackongo.Question type, and added to an array
// to be added into the database using the AddQuestions function in backend/databasing.go
func addNewQuestionToDatabaseHandler(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Infof(ctx, "%v", err)
	}
	var f interface{}
	err = json.Unmarshal(body, &f)
	if err != nil {
		log.Infof(ctx, "%v", err)
	}
	m := f.(map[string]interface{})
	question := m["Question"]
	state := m["State"]
	if err != nil {
		log.Infof(ctx, "%v", err)
	}
	var qn stackongo.Question
	json.Unmarshal([]byte(question.(string)), &qn)
	log.Infof(ctx, "%v", qn)

	user := getUser(w, r, ctx)
	log.Infof(ctx, "%v", user.User_id)

	if err := backend.AddSingleQuestion(db, qn, state.(string), user.User_id); err != nil {
		log.Warningf(ctx, "Error adding new question to db:\t", err)
	}
	backend.UpdateTableTimes(db, ctx, "question")
}

// Handler for keywords, tags, users in the search box
// Checks input against fields in the question/user caches and returns any matches
func searchHandler(w http.ResponseWriter, r *http.Request, ctx context.Context, pageNum int, user stackongo.User) {

	search := r.FormValue("search")

	query := "questions.question_id LIKE '" + search + "'" + // By question id
		" OR questions.question_url LIKE '" + search + "'" + // By url
		" OR questions.question_title LIKE '%" + search + "%'" + // By part of title
		" OR questions.body LIKE '%" + search + "%'" + // By part of body
		" OR (questions.user LIKE'" + search + "' AND questions.state!='unanswered')" + // By Owner id
		" OR (user.name LIKE '%" + search + "%' AND questions.state!='unanswered')" + // By Owner display name
		" OR questions.question_id IN (SELECT question_id FROM question_tag WHERE tag like '" + search + "')" // By tags
	tempData, updateTime, err := readFromDb(ctx, query)
	if err != nil {
		log.Errorf(ctx, "Error reading from db: %v", err.Error())
	} else {
		mostRecentUpdate = updateTime
	}

	page := template.Must(template.ParseFiles("public/template.html"))

	var pageQuery = []string{
		"search",
		search,
	}

	if err := page.Execute(w, writeResponse(user, tempData, pageNum, pageQuery)); err != nil {
		log.Errorf(ctx, "%v", err.Error())
	}

}

// Handler to find all questions with specific tags
func tagHandler(w http.ResponseWriter, r *http.Request, ctx context.Context, pageNum int, user stackongo.User) {
	// Collect query
	tag := r.FormValue("tagSearch")

	query := "questions.question_id IN (SELECT question_id FROM question_tag WHERE tag like '" + tag + "')" // By tags
	tempData, updateTime, err := readFromDb(ctx, query)
	if err != nil {
		log.Errorf(ctx, "Error reading from db: %v", err.Error())
	} else {
		mostRecentUpdate = updateTime
	}

	page := template.Must(template.ParseFiles("public/template.html"))
	var tagQuery = []string{
		"tag",
		tag,
	}
	if err := page.Execute(w, writeResponse(user, tempData, pageNum, tagQuery)); err != nil {
		log.Warningf(ctx, "%v", err.Error())
	}
}

// Handler to find all questions answered/being answered by the user in URL
func userHandler(w http.ResponseWriter, r *http.Request, ctx context.Context, pageNum int, user stackongo.User) {
	userID_string := r.FormValue("id")
	//userID_int, _ := strconv.Atoi(userID_string)
	query := userData{}

	// Create a new webData struct
	tempData, updateTime, err := readFromDb(ctx, "state='unanswered' OR question.user="+userID_string)
	if err != nil {
		log.Errorf(ctx, "Error reading from db: %v", err.Error())
	} else {
		mostRecentUpdate = updateTime
	}

	page := template.Must(template.ParseFiles("public/template.html"))
	var userQuery = []string{
		"user",
		query.User_info.Display_name,
	}
	if err := page.Execute(w, writeResponse(user, tempData, pageNum, userQuery)); err != nil {
		log.Warningf(ctx, "%v", err.Error())
	}
}

//Display a list of tags that are logged in the database
//User can either click on a tag to view any questions containing that tag
//Format array of tags into another array, to be easier formatted on the page into a table in the template
//An array of tagData arrays of size 4
func viewTagsHandler(w http.ResponseWriter, r *http.Request, ctx context.Context, pageNum int, user stackongo.User) {
	query := readTagsFromDb(ctx)
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
	first := (pageNum - 1) * 5
	last := pageNum * 5
	lastPage := len(tagArray) / 5
	if len(tagArray)%5 != 0 {
		lastPage++
	}
	if last > len(tagArray) {
		last = len(tagArray)
	}
	if err := page.Execute(w, queryReply{user, mostRecentUpdate, pageNum, lastPage, tagArray[first:last]}); err != nil {
		log.Warningf(ctx, "%v", err.Error())
	}

}

// Handler for viewing all users in the database
// Formats the response into an array of userData maps, for easier formatting onto the page.
// User data is stored as a map, which gives no guarantee as to the order of iteration
// It is first read into an array, and that array sorted lexicographically by the users Display name.
func viewUsersHandler(w http.ResponseWriter, r *http.Request, ctx context.Context, pageNum int, user stackongo.User) {
	query := readUsersFromDb(ctx, "")
	var querySorted []userData
	for id, i := range query {
		if id != user.User_id {
			querySorted = append(querySorted, i)
		}
	}
	sort.Sort(ByDisplayName(querySorted))

	var queryArray [][]userData
	var tempQueryArray []userData

	for i, u := range querySorted {
		tempQueryArray = append(tempQueryArray, u)
		if (i != 0 && i%4 == 0) || i+1 == len(querySorted) {
			queryArray = append(queryArray, tempQueryArray)
			//clear temp array
			tempQueryArray = nil
		}
	}
	final := struct {
		User   userData
		Others [][]userData
	}{
		query[user.User_id],
		queryArray,
	}
	log.Infof(ctx, "Current user = %v", final.User)
	log.Infof(ctx, "Other users:\n%v", final.Others)
	page := template.Must(template.ParseFiles("public/viewUsers.html"))
	if err := page.Execute(w, queryReply{user, mostRecentUpdate, pageNum, 0, final}); err != nil {
		log.Errorf(ctx, "%v", err.Error())
	}
}

func userPageHandler(w http.ResponseWriter, r *http.Request, ctx context.Context, data webData, pageNum int, user stackongo.User) {
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
	if err := page.Execute(w, queryReply{user, mostRecentUpdate, pageNum, 0, query}); err != nil {
		log.Errorf(ctx, "%v", err.Error())
	}
}

// Returns the current user requesting the page
func getUser(w http.ResponseWriter, r *http.Request, ctx context.Context) stackongo.User {
	// Collect userId from browser cookie
	username, err := r.Cookie("user_name")
	if err == nil && username.Value != "" && username.Value != "Guest" {
		return readUserFromDb(ctx, username.Value)
	}

	// If user_id cookie is not set, look for code in url request to collect access token.
	// If code is not available, return guest user
	code := r.FormValue("code")
	if code == "" {
		log.Infof(ctx, "Returning guest user")
		return guest
	}

	queries := r.URL.Query()
	queries.Del("code")
	r.URL.RawQuery = queries.Encode()
	// Collect access token using the recieved code
	access_tokens, err := backend.ObtainAccessToken(code, r.URL.String())
	if err != nil {
		log.Warningf(ctx, "Access token not obtained: %v", err.Error())
		return guest
	}

	// Get the authenticated user with the collected access token
	user, err := backend.AuthenticatedUser(map[string]string{}, access_tokens["access_token"])
	if err != nil {
		log.Warningf(ctx, err.Error())
		return guest
	}

	// Add user to db if not already in
	addUserToDB(ctx, user)

	//zhu li do the thing
	//updateLoginTime(ctx, user)

	return user
}

// Update the database if the lastPullTime is more than 6 hours before the current time
func updateDB(db *sql.DB, ctx context.Context, lastPullTime int64) int64 {
	// If the last pull was more than 6 hours ago
	if lastPull < time.Now().Add(-1*timeout).Unix() {
		log.Infof(ctx, "Updating database")

		// Remove deleted questions from the database
		log.Infof(ctx, "Removing deleted questions from db")
		if err := backend.RemoveDeletedQuestions(db, ctx); err != nil {
			log.Warningf(ctx, "Error removing deleted questions: %v", err.Error())
			return lastPullTime
		}

		// Setting time frame to get new questions.
		toDate := time.Now()
		fromDate := time.Unix(lastPull, 0)

		// Collect new questions from SO
		questions, err := backend.GetNewQns(fromDate, toDate)
		if err != nil {
			log.Warningf(ctx, "Error getting new questions: %v", err.Error())
			return lastPullTime
		}

		// Add new questions to database
		log.Infof(ctx, "Adding new questions to db")
		if err := backend.AddQuestions(db, ctx, questions); err != nil {
			log.Warningf(ctx, "Error adding new questions: %v", err.Error())
			return lastPullTime
		}

		lastPullTime = time.Now().Unix()
		log.Infof(ctx, "New questions added")
	}
	return lastPullTime
}

// Write a genReply struct with the inputted Question slices
// This can call readFromDb() now as a method, most of this is redundant.
func writeResponse(user stackongo.User, writeData webData, pageNum int, query []string) genReply {
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
		User:       user,             // Current user information
		Qns:        writeData.Qns,    // Map users by questions answered
		UpdateTime: mostRecentUpdate, // Time of last update
		Query:      query,            // Current query value
	}
}

// Handler for errors
func errorHandler(w http.ResponseWriter, r *http.Request, ctx context.Context, status int, err string) {
	w.WriteHeader(status)
	switch status {
	case http.StatusNotFound:
		page := template.Must(template.ParseFiles("public/404.html"))
		if err := page.Execute(w, nil); err != nil {
			errorHandler(w, r, ctx, http.StatusInternalServerError, err.Error())
			return
		}
	case http.StatusInternalServerError:
		w.Write([]byte("Internal error: " + err))
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
func newUser(u stackongo.User) userData {
	return userData{
		User_info: u,
		Caches: map[string][]stackongo.Question{
			"answered": []stackongo.Question{},
			"pending":  []stackongo.Question{},
			"updating": []stackongo.Question{},
		},
	}
}

// Returns the smaller value
func Min(x int, y int) int {
	if x < y {
		return x
	} else {
		return y
	}
}
