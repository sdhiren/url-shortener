package main

import (
	"database/sql"
	"fmt"
	"log"
	"urlshortener/router"
	"urlshortener/src/config"

	_ "github.com/lib/pq"
)

func main() {

	// var db *sql.DB

	fmt.Println("starting service")

	dbConfig, err := config.LoadDBConfig()
	if err != nil {
		log.Fatal("Invalid database configuration: ", err)
	}

	db, err := sql.Open("postgres", dbConfig.DSN())
	if err != nil {
		log.Fatal("Error connecting to the database:", err)
	}
	defer db.Close()

	// Ping the database to verify the connection
	err = db.Ping()
	if err != nil {
		log.Fatal("Error pinging the database:", err)
	}

	r := router.InitRouter(db)
	r.Run(":8080")
}
