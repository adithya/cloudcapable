package main

import (
	"fmt"
	"github.com/cloudcapable/terraformrunner"
	"net/http"
	"time"
)

func terraformRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		w.Write([]byte("Method Not Allowed"))
		return
	}

	terraformInput := r.PostFormValue("terraformInput")
	planOutput, err := terraformrunner.TerraformRunner(terraformInput)

	// just panic for now
	if err != nil {
		panic(err)
	}

	w.Write([]byte(planOutput))
}

func main() {
	runner := http.NewServeMux()
	runner.HandleFunc("/terraform", terraformRun)

	mux := http.NewServeMux()
	mux.HandleFunc("/GetVersion", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("0.1\n"))
	})
	mux.Handle("/run/", http.StripPrefix("/run", runner))

	s := http.Server{
		Addr:         ":8080",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 90 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      mux,
	}

	fmt.Printf("Starting server at :8080")
	err := s.ListenAndServe()
	if err != nil {
		if err != http.ErrServerClosed {
			panic(err)
		}
	}
}
