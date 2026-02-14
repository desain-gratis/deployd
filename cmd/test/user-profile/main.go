package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
)

func main() {
	router := httprouter.New()

	router.GET("/", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		fmt.Fprint(w, "User profile service is running.\n")
		key := "MESSAGE"
		fmt.Fprintf(w, "Env read: %v = %v.\n", key, os.Getenv(key))
	})

	server := &http.Server{
		Addr:    "0.0.0.0:10001",
		Handler: router,
	}

	log.Println("User profile service is running at http://0.0.0.0:10001")

	server.ListenAndServe()
}
