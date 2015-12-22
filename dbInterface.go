package webui

import (
	"database/sql"
	"log"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"appengine"

	"github.com/laktek/Stack-on-Go/stackongo"
)

func refreshCache() {
	tempData := readFromDb("")

	data.CacheLock.Lock()
	data.UnansweredCache = tempData.UnansweredCache
	data.AnsweredCache = tempData.AnsweredCache
	data.PendingCache = tempData.PendingCache
	data.UpdatingCache = tempData.UpdatingCache
	data.Qns = tempData.Qns
	data.CacheLock.Unlock()
}

func readFromDb(queries string) webData {
	//Reading from database
	log.Println("Refreshing database read")
	tempData := newWebData()
	var (
		url           string
		title         string
		id            int
		state         string
		body          string
		owner         sql.NullInt64
		name          sql.NullString
		image         sql.NullString
		link          sql.NullString
		creation_time int
	)
	//Select all questions in the database and read into a new data object
	query := "SELECT * FROM questions LEFT JOIN user ON questions.user=user.id"
	if queries != "" {
		query = query + " WHERE state='unanswered' OR " + queries
	}
	rows, err := db.Query(query)
	if err != nil {
		log.Fatal("query failed:\t", err)
	}

	defer rows.Close()
	//Iterate through each row and add to the correct cache
	for rows.Next() {
		err := rows.Scan(&id, &title, &url, &state, &owner, &body, &owner, &name, &image, &link, &creation_time)
		currentQ := stackongo.Question{
			Question_id: id,
			Title:       title,
			Link:        url,
			Body:        body,
		}
		if err != nil {
			log.Fatal("query failed:\t", err)
		}

		var tagToAdd string
		//Get tags for that question, based on the ID
		tagRows, err := db.Query("SELECT tag from question_tag where question_id = ?", currentQ.Question_id)
		if err != nil {
			log.Fatal("Tag retrieval failed!\t", err)
		}
		defer tagRows.Close()
		for tagRows.Next() {
			err := tagRows.Scan(&tagToAdd)
			if err != nil {
				log.Fatal("Could not scan for tag!\t", err)
			}
			currentQ.Tags = append(currentQ.Tags, tagToAdd)
		}
		//Switch on the state as read from the database to ensure question is added to correct cace
		switch state {
		case "unanswered":
			tempData.UnansweredCache = append(tempData.UnansweredCache, currentQ)
		case "answered":
			tempData.AnsweredCache = append(tempData.AnsweredCache, currentQ)
		case "pending":
			tempData.PendingCache = append(tempData.PendingCache, currentQ)
		case "updating":
			tempData.UpdatingCache = append(tempData.UpdatingCache, currentQ)
		}
		if owner.Valid {
			tempData.Qns[id] = stackongo.User{
				User_id:       int(owner.Int64),
				Display_name:  name.String,
				Profile_image: name.String,
			}
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
	return last_updated > mostRecentUpdate
}

func readUserFromDb(id string) stackongo.User {
	//Reading from database
	log.Println("Refreshing database read")
	var (
		owner sql.NullInt64
		name  sql.NullString
		image sql.NullString
		link  sql.NullString
	)
	//Select all questions in the database and read into a new data object
	rows, err := db.Query("SELECT * FROM user WHERE id=" + id)
	if err != nil {
		log.Fatal("query failed:\t", err)
	}

	defer rows.Close()
	//Iterate through each row and add to the correct cache
	for rows.Next() {
		err := rows.Scan(&owner, &name, &image, &link)
		if err != nil {
			log.Fatal("query failed:\t", err)
		}

		if owner.Valid {
			return stackongo.User{
				User_id:       int(owner.Int64),
				Display_name:  name.String,
				Profile_image: name.String,
			}
		}
	}
	return stackongo.User{}
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

// Write user data into the database
func addUser(newUser stackongo.User) {

	stmts, err := db.Prepare("INSERT IGNORE INTO user (id, name, pic) VALUES (?, ?, ?)")
	if err != nil {
		log.Println("Prepare failed:\t", err)
	}

	_, err = stmts.Exec(newUser.User_id, newUser.Display_name, newUser.Profile_image)
	if err != nil {
		log.Fatal("Insertion of new user failed:\t", err)
	}
}

// updating the caches based on input from the appi
func updatingCache_User(r *http.Request, c appengine.Context, user stackongo.User) error {
	c.Infof("updating cache")
	if checkDBUpdateTime("questions") /* time on sql db is later than lastUpdatedTime */ {
		mostRecentUpdate = int32(time.Now().Unix())
	}

	// required to collect post form data
	r.ParseForm()

	// Collect the submitted form info based on the name of the form
	// Check each cache against the form data

	// Create a copy of Data as a reflect.Value
	dataCopy := reflect.ValueOf(data)
	// Updated data
	newData := newWebData()
	// Question IDs of questions that have been updated
	// Maps IDs to new states
	changedQns := map[int]string{}

	// Range through the fields of the data copy
	for i := 0; i < dataCopy.NumField(); i++ {
		field := dataCopy.Field(i)
		if field.Type().String() == "[]stackongo.Question" {

			// Get the prefix of the form names
			cacheType := strings.ToLower(strings.TrimSuffix(dataCopy.Type().Field(i).Name, "Cache")) + "_"

			// Range through the array of the caches
			for j := 0; j < field.Len(); j++ {

				// Collect the question from the slice, changing it to the correct type
				question := field.Index(j).Interface().(stackongo.Question)
				var newState string

				// Get the full form names
				name := cacheType + strconv.Itoa(question.Question_id)
				// Collect form from Request
				form_input := r.PostFormValue(name)

				// Add the question to the appropriate cache, updating the state
				switch form_input {
				case "unanswered":
					newData.UnansweredCache = append(newData.UnansweredCache, question)
					newState = "unanswered"
				case "answered":
					newData.AnsweredCache = append(newData.AnsweredCache, question)
					newState = "answered"
				case "pending":
					newData.PendingCache = append(newData.PendingCache, question)
					newState = "pending"
				case "updating":
					newData.UpdatingCache = append(newData.UpdatingCache, question)
					newState = "updating"
				case "no_change":
					// If there has been no change, add the question back into the cache it was originally in
					newState = strings.Split(name, "_")[0]
					switch newState {
					case "unanswered":
						newData.UnansweredCache = append(newData.UnansweredCache, question)
					case "answered":
						newData.AnsweredCache = append(newData.AnsweredCache, question)
					case "pending":
						newData.PendingCache = append(newData.PendingCache, question)
					case "updating":
						newData.UpdatingCache = append(newData.UpdatingCache, question)
					}
				}

				// Update any info on the updated question
				if form_input != "" && form_input != "no_change" {
					changedQns[question.Question_id] = newState
					if form_input != "unanswered" {
						data.Qns[question.Question_id] = user
					}
				}
			}
		}
	}

	sort.Sort(byCreationDate(newData.UnansweredCache))
	sort.Sort(byCreationDate(newData.AnsweredCache))
	sort.Sort(byCreationDate(newData.PendingCache))
	sort.Sort(byCreationDate(newData.UpdatingCache))

	data.CacheLock.Lock()
	data.UnansweredCache = newData.UnansweredCache
	data.AnsweredCache = newData.AnsweredCache
	data.PendingCache = newData.PendingCache
	data.UpdatingCache = newData.UpdatingCache
	data.CacheLock.Unlock()

	//Update the table on SQL keeping track of table modifications
	updateTableTimes("questions")

	// Update the database
	go func(qns map[int]string, userId int) {
		updateDb(qns, userId)
	}(changedQns, user.User_id)
	return nil
}

// Fucntion to update the questions in qns in the database
func updateDb(qns map[int]string, userId int) {
	log.Println("Updating database")

	if len(qns) == 0 {
		return
	}

	query := "SELECT question_id, state, user FROM questions WHERE "
	// Add questions to update to the query
	for id, _ := range qns {
		query += "question_id=" + strconv.Itoa(id) + " OR "
	}
	query = strings.TrimSuffix(query, " OR ")

	// Pull the required questions from the database
	rows, err := db.Query(query)
	if err != nil {
		log.Printf("query failed:\t%v", err)
	}
	defer func() {
		log.Println("closing rows: updating")
		rows.Close()
	}()

	var (
		id    int
		owner int
		state string
	)

	for rows.Next() {
		err := rows.Scan(&id, &state, &owner)
		if err != nil {
			log.Printf("rows.Scan: %v", err.Error())
		}

		//Update the database, setting the state and the new user/owner of that question.
		stmts, err := db.Prepare("UPDATE questions SET state=?,user=? where question_id=?")
		if err != nil {
			log.Printf("%v", err.Error())
		}
		if qns[id] == "unanswered" {
			userId = 0
		}

		_, err = stmts.Exec(qns[id], userId, id)
		if err != nil {
			log.Printf("Update query failed:\t%v", err.Error())
		}
	}
	log.Println("Database updated")
}
