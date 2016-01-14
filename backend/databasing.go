package backend

import (
	"database/sql"
	"log"

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
	for _, item := range newQns.Items {
		//INSERT IGNORE ensures that the same question won't be added again
		stmt, err := db.Prepare("INSERT IGNORE INTO questions(question_id, question_title, question_URL, body, creation_date) VALUES (?, ?, ?, ?, ?)")
		if err != nil {
			return err
		}
		_, err = stmt.Exec(item.Question_id, item.Title, item.Link, StripTags(item.Body), item.Creation_date)
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
