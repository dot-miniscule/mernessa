package webui

import (
	"backend"
	"database/sql"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/laktek/Stack-on-Go/stackongo"
)

func initCacheDownload() {
	data.CacheLock.Lock()
	data.Users = readUsersFromDb()
	data.CacheLock.Unlock()
	refreshLocalCache()
}

func refreshLocalCache() {
	tempData := readFromDb("")

	data.CacheLock.Lock()
	for cacheType, _ := range tempData.Caches {
		data.Caches[cacheType] = tempData.Caches[cacheType]
	}
	data.Qns = tempData.Qns
	for id, info := range tempData.Users {
		data.Users[id] = info
	}
	data.CacheLock.Unlock()
}

func readFromDb(queries string) webData {
	//Reading from database
	log.Println("Refreshing database read")
	tempData := newWebData()
	var (
		url            string
		title          string
		id             int
		state          string
		body           string
		creation_date  int64
		last_edit_time sql.NullInt64
		owner          sql.NullInt64
		name           sql.NullString
		pic            sql.NullString
		link           sql.NullString
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
		err := rows.Scan(&id, &title, &url, &state, &owner, &body, &creation_date, &last_edit_time, &owner, &name, &pic, &link)
		currentQ := stackongo.Question{
			Question_id:   id,
			Title:         title,
			Link:          url,
			Body:          body,
			Creation_date: creation_date,
		}
		if last_edit_time.Valid {
			currentQ.Last_edit_date = last_edit_time.Int64
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
				Profile_image: pic.String,
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

	mostRecentUpdate = time.Now().Unix()
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
func readUsersFromDb() map[int]userData {

	log.Println("Retrieving users from db")

	tempData := make(map[int]userData)

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

		currentUser := stackongo.User{
			User_id:       int(id.Int64),
			Display_name:  name.String,
			Profile_image: pic.String,
			Link:          link.String,
		}
		tempData[int(id.Int64)] = newUser(currentUser, "")
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
func checkDBUpdateTime(tableName string, lastUpdate int64) bool {
	var (
		last_updated int64
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
func updatingCache_User(r *http.Request, user stackongo.User) error {
	log.Printf("updating cache")

	mostRecentUpdate = time.Now().Unix()

	// required to collect post form data
	r.ParseForm()

	// Updated data
	newData := data

	// Question IDs of questions that have been updated
	// Maps IDs to new states
	changedQns := map[int]string{}
	changedQnsTitles := []string{}

	// Collect the submitted form info based on the name of the form
	// Check each cache against the form data
	for cacheType, cache := range data.Caches {

		// Range through the array of the caches
		for _, question := range cache {

			// Get the full form names
			questionID := cacheType + "_" + strconv.Itoa(question.Question_id)
			// Collect form from Request
			form_input := r.PostFormValue(questionID)
			// Add the question to the appropriate cache, updating the state
			if _, ok := newData.Caches[form_input]; ok {
				question.Last_edit_date = mostRecentUpdate
				newData.Caches[form_input] = append(newData.Caches[form_input], question)
				for i := 0; i < len(newData.Caches[cacheType]); i++ {
					if newData.Caches[cacheType][i].Question_id == question.Question_id {
						newData.Caches[cacheType] = append(newData.Caches[cacheType][:i], newData.Caches[cacheType][i+1:]...)
						break
					}
				}

				// Update any info on the updated question
				changedQns[question.Question_id] = form_input
				changedQnsTitles = append(changedQnsTitles, question.Title)
				if form_input != "unanswered" {
					newData.Qns[question.Question_id] = user
					if _, ok := newData.Users[user.User_id]; !ok {
						newData.Users[user.User_id] = newUser(user, "")
					}

					newData.Users[user.User_id].Caches[form_input] = append(newData.Users[user.User_id].Caches[form_input], question)
				}
			}
		}
	}

	for cacheType, _ := range newData.Caches {
		sort.Sort(byCreationDate(newData.Caches[cacheType]))
	}

	data.CacheLock.Lock()
	data.Qns = newData.Qns
	data.Users = newData.Users
	for cacheType, _ := range newData.Caches {
		data.Caches[cacheType] = newData.Caches[cacheType]
	}
	data.CacheLock.Unlock()

	// Update the database
	go func(db *sql.DB, qns map[int]string, qnsTitles []string, userId int, lastUpdate int64) {
		recentChangedQns = qnsTitles
		backend.UpdateDb(db, qns, userId, lastUpdate)
	}(db, changedQns, changedQnsTitles, user.User_id, mostRecentUpdate)
	return nil
}
