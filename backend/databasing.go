package backend

import (
	"dataCollect"
	"database/sql"
	"encoding/json"
	"html"
	"log"
	//"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/laktek/Stack-on-Go/stackongo"
	"golang.org/x/net/context"
	applog "google.golang.org/appengine/log"
)

// Information for a sql database
type databaseInfo struct {
	username string
	dbName   string
	password string
	IP       string
}

//Create database connection & connection pool
//Once opened this does not need to be called again
//sql.DB ISNT A DATABASE CONNECTION, its an abstraction of the interface.
//It opens and closes connections to the underlying database
//and manages a pool of connections as needed
//returns a *sql.DB to query elsewhere.
func SqlInit(dbString string) *sql.DB {
	db, err := sql.Open("mysql", dbString)
	if err != nil {
		log.Println("Open fail: \t", err)
		return nil
	}

	//Usually would defer the closing of the database connection from here
	//Assuming this function is called within another method, it will need to be closed at the
	//return of that function --> db.Close()
	log.Println("Pinging the database. This may take a moment...")

	//Verify data source is valid
	err = db.Ping()
	if err != nil {
		log.Println("Ping failed: \t", err)
	} else {
		log.Println("Database initialized successfully!")
	}

	//Return the db pointer for use elsewhere, as it has now been successfully created
	return db
}

// This function checks if an existing question is already present in the database, based on ID
// If so, doing a call to the StackExchange API is useless, and a waste of our daily quota
// SELECT EXIST returns a single row with a 1 or 0 depending on whether or not a record exists
func CheckForExistingQuestion(db *sql.DB, id int) (int, error) {
	res := 0
	rows, err := db.Query("SELECT EXISTS(SELECT * FROM questions where question_id=?)", id)
	if err != nil {
		return res, err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&res)
		if err != nil {
			return res, err
		}
	}
	err = rows.Err()
	if err != nil {
		return res, err
	}

	return res, nil
}

// Given a question ID, it pulls that from the database
// Marshalls the result as JSON data to be returned in a reply
// Checks if a question is unanswered, if not it pulls the display name for that user
func PullQnByID(db *sql.DB, ctx context.Context, id int) []byte {

	type newQ struct {
		Message string

		Question_id   int
		Creation_date int64
		Link          string
		Body          string
		Title         string
		Tags          []string

		State           string
		UserID          string
		UserDisplayName string
		Time            string
	}

	rows, err := db.Query("SELECT * FROM questions where question_id=?", id)
	if err != nil {
		applog.Warningf(ctx, "Question query failed: %v", err.Error())
		return []byte{}
	}

	defer rows.Close()
	var n newQ
	n.Message = "Question already exists in database. See below."
	for rows.Next() {
		var sqlTime sql.NullInt64
		var t int64
		err := rows.Scan(&n.Question_id, &n.Title, &n.Link, &n.State, &n.UserID, &n.Body, &n.Creation_date, &sqlTime)
		if err != nil {
			applog.Errorf(ctx, "Question scan failed: %v", err.Error())
			continue
		}
		if sqlTime.Valid {
			t = sqlTime.Int64
			n.Time = time.Unix(t, 0).Format("Jan 2 at 15:04")
		} else {
			n.Time = ""
		}
		if n.State != "unanswered" {
			userRows, err := db.Query("SELECT name FROM user WHERE id=?", n.UserID)
			if err != nil {
				log.Println(err)
			}
			defer userRows.Close()
			for userRows.Next() {
				err := userRows.Scan(&n.UserDisplayName)
				if err != nil {
					log.Println(err)
				}
			}
			err = userRows.Err()
			if err != nil {
				log.Println(err)
			}

		} else {
			n.UserDisplayName = ""
		}

		tagRows, err := db.Query("SELECT tag from question_tag where question_id = ?", id)
		if err != nil {
			applog.Errorf(ctx, "Tag query failed: %v", err.Error())
			continue
		}
		defer tagRows.Close()
		var currentTag string
		for tagRows.Next() {
			err := tagRows.Scan(&currentTag)
			if err != nil {
				applog.Errorf(ctx, "Tag scan failed: %v", err.Error())
				continue
			}
			n.Tags = append(n.Tags, currentTag)
		}
	}
	err = rows.Err()
	if err != nil {
		applog.Errorf(ctx, "Error during row iteration: %v", err.Error())
	}
	b, err := json.Marshal(n)
	if err != nil {
		applog.Errorf(ctx, "Marshaling failed: %v", err.Error())
		return []byte{}
	}
	return b
}

// Adds a single question into the database, accepting a question, the questions state
// and the users id. The state will determine whether or not it needs to be timestamped.
// Returns an error on fail
func AddSingleQuestion(db *sql.DB, item stackongo.Question, state string, user int) error {
	if state == "pending" || state == "answered" || state == "updating" {
		//INSERT IGNORE ensures that the same question won't be added again
		stmt, err := db.Prepare("INSERT IGNORE INTO questions(question_id, question_title, question_URL, body, creation_date, state, user, time_updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
		if err != nil {
			return err
		}
		_, err = stmt.Exec(item.Question_id, html.UnescapeString(item.Title), item.Link, html.UnescapeString(StripTags(item.Body)), item.Creation_date, state, user, time.Now().Unix())
		if err != nil {
			log.Println("Exec insertion for question failed!:\t", err)
			return err
		}
	} else {
		//INSERT IGNORE ensures that the same question won't be added again
		stmt, err := db.Prepare("INSERT IGNORE INTO questions(question_id, question_title, question_URL, body, creation_date, state, user) VALUES (?, ?, ?, ?, ?, ?, ?)")
		if err != nil {
			return err
		}
		_, err = stmt.Exec(item.Question_id, html.UnescapeString(item.Title), item.Link, html.UnescapeString(StripTags(item.Body)), item.Creation_date, state, user)
		if err != nil {
			log.Println("Exec insertion for question failed!:\t", err)
			return err
		}
	}

	for _, tag := range item.Tags {
		stmt, err := db.Prepare("INSERT IGNORE INTO question_tag(question_id, tag) VALUES(?, ?)")
		if err != nil {
			return err
		}

		_, err = stmt.Exec(item.Question_id, tag)
		if err != nil {
			return err
		}
	}
	return nil
}

// Adds a set of questions into the database, by calling the AddSingleQuestions function
// Default state for new questions is unanswered, with a user ID of 0.
func AddQuestions(db *sql.DB, ctx context.Context, newQns *stackongo.Questions) error {

	for _, item := range newQns.Items {
		err := AddSingleQuestion(db, item, "unanswered", 0)
		if err != nil {
			applog.Errorf(ctx, "Error adding question %v: %v", item.Question_id, err.Error())
		}
	}
	UpdateTableTimes(db, ctx, "question")
	return nil
}

// Remove any questions in db that does not exist in StackOverflow
func RemoveDeletedQuestions(db *sql.DB, ctx context.Context) error {
	defer UpdateTableTimes(db, ctx, "questions")

	// Get questions from db
	ids := []int{}
	rows, err := db.Query("SELECT question_id FROM questions")
	if err != nil {
		return err
	}
	var id int
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&id)
		if err != nil {
			return err
		}
		ids = append(ids, id)
	}

	// Get the questions from StackExchange
	params := make(stackongo.Params)
	params.Pagesize(100)
	params.Sort("creation")
	params.AddVectorized("tagged", tags)

	questions, err := dataCollect.GetQuestionsByIDs(session, ids, appInfo, params)
	if err != nil {
		return err
	}

	// If the number of questions equal, no questions have been deleted
	if len(questions.Items) == len(ids) {
		return nil
	}

	// Find the questions not in SE
	// Add to deletedQns
	deletedQns := make([]int, 0, len(ids)-len(questions.Items))
	for _, id := range ids {
		deleted := true
		for _, question := range questions.Items {
			if question.Question_id == id {
				deleted = false
				break
			}
		}
		if deleted {
			deletedQns = append(deletedQns, id)
		}
	}

	// Remove deletedQns from db
	query := "DELETE FROM questions WHERE "
	for i, id := range deletedQns {
		query += "question_id=" + strconv.Itoa(id)
		if i < len(deletedQns)-1 {
			query += " OR "
		}
	}
	_, err = db.Exec(query)
	if err != nil {
		return err
	}

	// Remove tags associated with deleted questions
	_, err = db.Exec("DELETE FROM question_tag WHERE question_id NOT IN (SELECT questions.question_id FROM questions)")
	if err != nil {
		return err
	}

	return nil
}

// A crude way to find out if the working cache needs to be refreshed from the database.
// Stores the current Unix time in update_times table on Cloud SQL
func UpdateTableTimes(db *sql.DB, ctx context.Context, tableName string) {
	stmts, err := db.Prepare("UPDATE update_times SET last_updated=? WHERE table_name=?")
	if err != nil {
		applog.Errorf(ctx, "Prepare failed: %v", err)
		return
	}

	timeNow := time.Now().Unix()
	_, err = stmts.Exec(timeNow, tableName)
	if err != nil {
		applog.Errorf(ctx, "Could not update time for %v: %v", tableName, err)
	} else {
		applog.Infof(ctx, "Update time for %v successfully updated to %v!", tableName, timeNow)
	}
}

// Fucntion to update the questions in qns in the database
func UpdateQns(db *sql.DB, ctx context.Context, qns map[int]string, userId int, lastUpdate int64) {
	applog.Infof(ctx, "Updating database")

	if len(qns) == 0 {
		return
	}

	// Add questions to update to the query
	query := "SELECT question_id FROM questions WHERE "
	for id, _ := range qns {
		query += "question_id=" + strconv.Itoa(id) + " OR "
	}
	query = strings.TrimSuffix(query, " OR ")

	// Pull the required questions from the database
	rows, err := db.Query(query)
	if err != nil {
		applog.Errorf(ctx, "query failed: %v", err)
		return
	}
	defer func() {
		applog.Infof(ctx, "closing rows: updating")
		rows.Close()
	}()

	var id int

	for rows.Next() {
		err := rows.Scan(&id)
		if err != nil {
			applog.Errorf(ctx, "Question_id scan failed: %v", err.Error())
			continue
		}

		//Update the database, setting the state and the new user/owner of that question.
		stmts, err := db.Prepare("UPDATE questions SET state=?,user=?,time_updated=? where question_id=?")
		if err != nil {
			applog.Errorf(ctx, "Update prepare failed: %v", err.Error())
			continue
		}
		if qns[id] == "unanswered" {
			userId = 0
		}

		_, err = stmts.Exec(qns[id], userId, lastUpdate, id)
		if err != nil {
			applog.Errorf(ctx, "Update execution failed:\t%v", err.Error())
		}
	}

	//Update the table on SQL keeping track of table modifications
	UpdateTableTimes(db, ctx, "questions")
	applog.Infof(ctx, "Database updated")
}
