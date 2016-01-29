package backend

import (
	"dataCollect"
	"database/sql"
	"encoding/json"
	"html"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/laktek/Stack-on-Go/stackongo"
)

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
func SqlInit() *sql.DB {
	//ipv6:= "tcp(" + net.JoinHostPort("2001:4860:4864:1:aebb:124d:884e:3108", "3306") + ")"
	//log.Println("JoinHostPort -", ipv6)
	/*sqldb := databaseInfo{
		"root",
		"mernessa",
		"password",
		"tcp(173.194.225.82:3306)",
	} */
	// log.Println(appengine.VersionID(ctx))
	//TODO: MEREDITH change to ipv6 address so ipv4 can be released on cloud sql.
	//		Also, update logging for appengine context.
	//db, err := sql.Open("mysql", "root@cloudsql(google.com:test-helloworld-1151:storage)/mernessa")
	dbString := os.Getenv("DB_STRING")
	db, err := sql.Open("mysql", dbString)
	if err != nil {
		log.Println("Open fail: \t", err)
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
func PullQnByID(db *sql.DB, id int) []byte {

	type newQ struct {
		Message string

		Question_id   int
		Creation_date int64
		Link          string
		Body          string
		Title         string
		Tags          []string

		State string
		Owner string
		Time  sql.NullInt64
	}

	rows, err := db.Query("SELECT * FROM questions where question_id=?", id)
	if err != nil {
		log.Println(err)
	}
	defer rows.Close()
	var n newQ
	n.Message = "Question already exists in database. See below."
	for rows.Next() {
		err := rows.Scan(&n.Question_id, &n.Title, &n.Link, &n.State, &n.Owner, &n.Body, &n.Creation_date, &n.Time)
		if err != nil {
			log.Println(err)
		}

		tagRows, err := db.Query("SELECT tag from question_tag where question_id = ?", id)
		if err != nil {
			log.Println(err)
		}
		defer tagRows.Close()
		var currentTag string
		for tagRows.Next() {
			err := tagRows.Scan(&currentTag)
			if err != nil {
				log.Println(err)
			}
			n.Tags = append(n.Tags, currentTag)
		}
	}
	err = rows.Err()
	if err != nil {
		log.Println(err)
	}
	b, err := json.Marshal(n)
	if err != nil {
		log.Println(b, err)
	}
	log.Println(n)
	return b
}

func AddSingleQuestion(db *sql.DB, item stackongo.Question, state string) error {

	//INSERT IGNORE ensures that the same question won't be added again
	stmt, err := db.Prepare("INSERT IGNORE INTO questions(question_id, question_title, question_URL, body, creation_date, state) VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(item.Question_id, html.UnescapeString(item.Title), item.Link, html.UnescapeString(StripTags(item.Body)), item.Creation_date, state)
	if err != nil {
		log.Println("Exec insertion for question failed!:\t", err)
		return err
	}
	for _, tag := range item.Tags {
		stmt, err = db.Prepare("INSERT IGNORE INTO question_tag(question_id, tag) VALUES(?, ?)")
		if err != nil {
			log.Println("question_tag insertion failed!:\t", err)
			return err
		}

		_, err = stmt.Exec(item.Question_id, tag)
		if err != nil {
			log.Println("Exec insertion for question_tag failed!:\t", err)
			return err
		}
	}
	return nil
}

func AddQuestions(db *sql.DB, newQns *stackongo.Questions) error {

	for _, item := range newQns.Items {
		AddSingleQuestion(db, item, "unanswered")
	}
	UpdateTableTimes(db, "question")
	return nil
}

func RemoveDeletedQuestions(db *sql.DB) error {
	defer UpdateTableTimes(db, "questions")
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

	if len(questions.Items) == len(ids) {
		return nil
	}

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
	_, err = db.Exec("DELETE FROM question_tag WHERE question_id NOT IN (SELECT questions.question_id FROM questions)")
	if err != nil {
		return err
	}

	return nil
}

//A crude way to find out if the working cache needs to be refreshed from the database.
//Stores the current Unix time in update_times table on Cloud SQL */
func UpdateTableTimes(db *sql.DB, tableName string) {
	stmts, err := db.Prepare("UPDATE update_times SET last_updated=? WHERE table_name=?")
	if err != nil {
		log.Println("Prepare failed:\t", err)
	}

	timeNow := time.Now().Unix()
	_, err = stmts.Exec(timeNow, tableName)
	if err != nil {
		log.Println("Could not update time for", tableName+":\t", err)
	} else {
		log.Printf("Update time for %v successfully updated to %v!", tableName, timeNow)
	}
}

// Fucntion to update the questions in qns in the database
func UpdateDb(db *sql.DB, qns map[int]string, userId int, lastUpdate int64) {
	log.Println("Updating database")

	if len(qns) == 0 {
		return
	}

	query := "SELECT question_id FROM questions WHERE "
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

	var id int

	for rows.Next() {
		err := rows.Scan(&id)
		if err != nil {
			log.Printf("rows.Scan: %v", err.Error())
		}

		//Update the database, setting the state and the new user/owner of that question.
		stmts, err := db.Prepare("UPDATE questions SET state=?,user=?,time_updated=? where question_id=?")
		if err != nil {
			log.Printf("%v", err.Error())
		}
		if qns[id] == "unanswered" {
			userId = 0
		}

		_, err = stmts.Exec(qns[id], userId, lastUpdate, id)
		if err != nil {
			log.Printf("Update query failed:\t%v", err.Error())
		}
	}

	//Update the table on SQL keeping track of table modifications
	UpdateTableTimes(db, "questions")
	log.Println("Database updated")
}
