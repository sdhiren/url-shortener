package main

import (
	"database/sql"
	"fmt"
	"log"
	"urlshortener/router"
	"urlshortener/src/config"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

func main() {

	// var db *sql.DB

	fmt.Println("starting service")

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Invalid configuration:\n", err)
	}
	gin.SetMode(cfg.Server.Mode)

	db, err := sql.Open("postgres", cfg.DB.DSN())
	if err != nil {
		log.Fatal("Error connecting to the database:", err)
	}
	defer db.Close()

	// Ping the database to verify the connection
	err = db.Ping()
	if err != nil {
		log.Fatal("Error pinging the database:", err)
	}

	r := router.InitRouter(db, cfg.Server.BaseURL)
	log.Printf("listening on %s, short links use %s", cfg.Server.Addr(), cfg.Server.BaseURL)
	if err := r.Run(cfg.Server.Addr()); err != nil {
		log.Fatal("Server stopped: ", err)
	}
}
