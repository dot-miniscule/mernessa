package webui

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"

	"appengine"

	"github.com/laktek/Stack-on-Go/stackongo"
)

func refreshCache() {
	tempData := readFromDb("")

	data.cacheLock.Lock()
	data.unansweredCache = tempData.unansweredCache
	data.answeredCache = tempData.answeredCache
	data.pendingCache = tempData.pendingCache
	data.updatingCache = tempData.updatingCache
	data.qns = tempData.qns
	data.cacheLock.Unlock()
}

func readFromDb(queries string) webData {
	//Reading from database
	log.Println("Refreshing database read")
	tempData := newWebData()
	var (
		url   string
		title string
		id    int
		state string
		owner sql.NullInt64
		name  sql.NullString
		image sql.NullString
		link  sql.NullString
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
		err := rows.Scan(&id, &title, &url, &state, &owner, &owner, &name, &image, &link)
		currentQ := stackongo.Question{
			Question_id: id,
			Title:       title,
			Link:        url,
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
			tempData.unansweredCache = append(tempData.unansweredCache, currentQ)
		case "answered":
			tempData.answeredCache = append(tempData.answeredCache, currentQ)
		case "pending":
			tempData.pendingCache = append(tempData.pendingCache, currentQ)
		case "updating":
			tempData.updatingCache = append(tempData.updatingCache, currentQ)
		}
		if owner.Valid {
			tempData.qns[id] = stackongo.User{
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
	/*	if checkDBUpdateTime("questions") /* time on sql db is later than lastUpdatedTime  {
		mostRecentUpdate = int32(time.Now().Unix())
	}*/

	// required to collect post form data
	r.ParseForm()

	// If the user is not in the database, add a new entry
	if _, ok := users[user.User_id]; !ok {
		users[user.User_id] = &userData{}
		users[user.User_id].init(user, "")
	}

	// Collect the submitted form info based on the name of the form
	// Check each cache against the form data
	// Read from db based on each question id (primary key) to retrieve and update the state
	var (
		url   string
		title string
		id    int
		numQ  int
		owner int
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
		err := rows.Scan(&id, &title, &url, &row.state, &owner)
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
			newState = "answered"
		case "pending":
			newState = "pending"
		case "updating":
			newState = "updating"
		case "no_change":
			newState = row.state
		}

		//Update the database, setting the state and the new user/owner of that question.
		if newState != row.state {
			stmts, err := db.Prepare("UPDATE questions SET state=?,user=? where question_id=?")
			if err != nil {
				c.Errorf("%v", err.Error())
			}
			id := 0
			if newState != "unanswered" {
				id = user.User_id
			}
			_, err = stmts.Exec(newState, id, row.question.Question_id)
			if err != nil {
				c.Errorf("Update query failed:\t%v", err.Error())
			}
		}
	}
	//Update the table on SQL keeping track of table modifications
	updateTableTimes("questions")
	refreshCache()
	return nil
}
