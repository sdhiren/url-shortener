package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"urlshortener/router"

	_ "github.com/lib/pq"
)


func main(){

	// var db *sql.DB

	fmt.Println("starting service")

	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")

	// Construct the database connection string
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPassword, dbHost, dbPort, dbName)

	db, err := sql.Open("postgres", connStr)
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