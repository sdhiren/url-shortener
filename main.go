package main

import "urlshortener/router"


func main(){
	r := router.InitRouter()
	r.Run(":8080")
}