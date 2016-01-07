package webui

import (
	"database/sql"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"appengine"

	"github.com/laktek/Stack-on-Go/stackongo"
)

func refreshLocalCache() {
	tempData := readFromDb("")

	data.CacheLock.Lock()
	for cacheType, _ := range tempData.Caches {
		data.Caches[cacheType] = tempData.Caches[cacheType]
	}
	data.Qns = tempData.Qns
	data.Users = tempData.Users
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
		creation_date int
		owner         sql.NullInt64
		name          sql.NullString
		image         sql.NullString
		link          sql.NullString
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
		err := rows.Scan(&id, &title, &url, &state, &owner, &body, &creation_date, &owner, &name, &image, &link)
		currentQ := stackongo.Question{
			Question_id:   id,
			Title:         title,
			Link:          url,
			Body:          body,
			Creation_date: int64(creation_date),
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
		tempData.Caches[state] = append(tempData.Caches[state], currentQ)

		if owner.Valid {
			user := stackongo.User{
				User_id:       int(owner.Int64),
				Display_name:  name.String,
				Profile_image: name.String,
			}
			tempData.Qns[id] = user
			if _, ok := tempData.Users[user.User_id]; !ok {
				tempData.Users[user.User_id] = newUser(user, "")
			}
			tempData.Users[user.User_id].Caches[state] = append(tempData.Users[user.User_id].Caches[state], currentQ)
		}
	}

	for cacheType, _ := range tempData.Caches {
		sort.Sort(byCreationDate(tempData.Caches[cacheType]))
	}

	mostRecentUpdate = int32(time.Now().Unix())
	return tempData
}

//Function called when the /viewTags request is made
//Retrieves all tags (which should be unique) and the number of questions saved in the db with that tag
func readTagsFromDb() []tagData {
	log.Println("Retrieving tags from db")
	var tempData []tagData

	var (
		tag   sql.NullString
		count sql.NullInt64
	)

	rows, err := db.Query("SELECT * FROM tags")
	if err != nil {
		log.Println("Tag query failed, ln 127:", err)
	}

	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tag, &count)
		if err != nil {
			log.Println("Scan failed, ln 134:", err)
		}
		currentTag := tagData{tag.String, int(count.Int64)}
		tempData = append(tempData, currentTag)
	}

	return tempData
}

//Function to read all user data from the database when a /viewUsers request is made
//Retrieves all users data

func readUsersFromDb() []userInfo {

	log.Println("Retrieving users from db")

	var tempData []userInfo

	var (
		id   sql.NullInt64
		name sql.NullString
		pic  sql.NullString
		link sql.NullString
	)

	rows, err := db.Query("SELECT * FROM user")
	if err != nil {
		log.Println("User query failed, ln 163:", err)
	}

	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&id, &name, &pic, &link)
		if err != nil {
			log.Println("User scan failed, ln 170:", err)
		}

		currentUser := userInfo{int(id.Int64), name.String, pic.String, link.String}
		tempData = append(tempData, currentUser)
	}

	return tempData

}

//Function to retrieve list of questions relating to particular user when a /userPage?xxx is made
func getUserQnsFromDb(userId string) []userInfo {
	var tempData []userInfo

	return tempData
}

/* Function to check if the DB has been updated since we last queried it
Returns true if our cache needs to be refreshed
False if is all g */
func checkDBUpdateTime(tableName string, lastUpdate int32) bool {
	var (
		last_updated int32
	)
	rows, err := db.Query("SELECT last_updated FROM update_times WHERE table_name='" + tableName + "'")
	if err != nil {
		log.Fatal("Query failed:\t", err)
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&last_updated)
		if err != nil {
			log.Fatal(err)
		}
	}
	if err != nil {
		log.Fatal(err)
	}
	return last_updated > lastUpdate
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

	timeNow := int32(time.Now().Unix())
	_, err = stmts.Exec(timeNow, tableName)
	if err != nil {
		log.Println("Could not update time for", tableName+":\t", err)
	} else {
		log.Printf("Update time for %v successfully updated to %v!", tableName, timeNow)
	}
}

// Write user data into the database
func addUserToDB(newUser stackongo.User) {

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

	// required to collect post form data
	r.ParseForm()

	// Collect the submitted form info based on the name of the form
	// Check each cache against the form data

	// Updated data
	newData := newWebData()
	// Question IDs of questions that have been updated
	// Maps IDs to new states
	changedQns := map[int]string{}

	for cacheType, cache := range data.Caches {

		// Range through the array of the caches
		for _, question := range cache {

			// Get the full form names
			questionID := cacheType + "_" + strconv.Itoa(question.Question_id)
			// Collect form from Request
			form_input := r.PostFormValue(questionID)

			// Add the question to the appropriate cache, updating the state
			if _, ok := newData.Caches[form_input]; ok {
				newData.Caches[form_input] = append(newData.Caches[form_input], question)

				// Update any info on the updated question
				changedQns[question.Question_id] = form_input
				if form_input != "unanswered" {
					newData.Qns[question.Question_id] = user
					if _, ok := newData.Users[user.User_id]; !ok {
						newData.Users[user.User_id] = newUser(user, "")
					}

					newData.Users[user.User_id].Caches[form_input] = append(newData.Users[user.User_id].Caches[form_input], question)
				}
			} else if form_input == "no_change" {
				// If there has been no change, add the question back into the cache it was originally in
				newData.Caches[cacheType] = append(newData.Caches[cacheType], question)
				if cacheType != "unanswered" {
					prevUser := data.Qns[question.Question_id]
					if _, ok := newData.Users[prevUser.User_id]; !ok {
						newData.Users[prevUser.User_id] = newUser(prevUser, "")
					}
					newData.Users[prevUser.User_id].Caches[cacheType] = append(newData.Users[prevUser.User_id].Caches[cacheType], question)
				}
			}
		}
	}

	for cacheType, _ := range newData.Caches {
		sort.Sort(byCreationDate(newData.Caches[cacheType]))
	}

	data.CacheLock.Lock()
	for cacheType, _ := range newData.Caches {
		data.Caches[cacheType] = newData.Caches[cacheType]
	}
	data.CacheLock.Unlock()

	// Update the database
	go func(qns map[int]string, userId int) {
		mostRecentUpdate = int32(time.Now().Unix())
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

	//Update the table on SQL keeping track of table modifications
	updateTableTimes("questions")
	log.Println("Database updated")
}
