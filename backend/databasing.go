package backend

import (
	"database/sql"
	"log"

	_ "github.com/go-sql-driver/mysql"
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
	sqldb := databaseInfo{
		"root",
		"mernessa",
		"password",
		"tcp(173.194.225.82:3306)",
	}

	dbString := "mysql" + sqldb.username + ":" + sqldb.password + "@" + sqldb.IP + "/" + sqldb.dbName
	log.Println("sql open url =", dbString)
	db, err := sql.Open("mysql", sqldb.username+":"+sqldb.password+"@"+sqldb.IP+"/"+sqldb.dbName)
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
