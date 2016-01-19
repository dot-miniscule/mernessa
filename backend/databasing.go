package backend

import (
	"dataCollect"
	"database/sql"
	"html"
	"log"
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
	//TODO: MEREDITH change to ipv6 address so ipv4 can be released on cloud sql.
	//		Also, update logging for appengine context.
	db, err := sql.Open("mysql", "root@cloudsql(google.com:test-helloworld-1151:storage)/mernessa")
	//db, err := sql.Open("mysql", "root:password@tcp(173.194.225.82:3306)/mernessa")
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

func AddQuestions(db *sql.DB, newQns *stackongo.Questions) error {

	defer UpdateTableTimes(db, "questions")
	for _, item := range newQns.Items {
		//INSERT IGNORE ensures that the same question won't be added again
		stmt, err := db.Prepare("INSERT IGNORE INTO questions(question_id, question_title, question_URL, body, creation_date) VALUES (?, ?, ?, ?, ?)")
		if err != nil {
			return err
		}
		_, err = stmt.Exec(item.Question_id, html.UnescapeString(item.Title), item.Link, html.UnescapeString(StripTags(item.Body)), item.Creation_date)
		if err != nil {
			log.Println("Exec insertion for question failed!:\t", err)
			continue
		}

		for _, tag := range item.Tags {
			stmt, err = db.Prepare("INSERT IGNORE INTO question_tag(question_id, tag) VALUES(?, ?)")
			if err != nil {
				log.Println("question_tag insertion failed!:\t", err)
				continue
			}

			_, err = stmt.Exec(item.Question_id, tag)
			if err != nil {
				log.Println("Exec insertion for question_tag failed!:\t", err)
				continue
			}
		}
	}
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

	log.Println(time.Now().Unix())
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
