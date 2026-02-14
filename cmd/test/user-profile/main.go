package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func main() {
	router := httprouter.New()

	router.GET("/", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		fmt.Fprint(w, "User profile service is running\n")
	})

	server := &http.Server{
		Addr:    "0.0.0.0:10001",
		Handler: router,
	}

	log.Println("User profile service is running at http://0.0.0.0:10001")

	server.ListenAndServe()
}
