package main

import (
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

var (
	tmpl *template.Template
)

func main() {
	var err error
	tmpl, err = template.ParseFiles("index.html")
	if err != nil {
		log.Fatal(err)
	}
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.StripSlashes)
	r.Get("/", handler)
	http.ListenAndServe(":8080", r)
}

type snippet struct {
	Def string
}

func handler(w http.ResponseWriter, r *http.Request) {
	tmpl.Lookup("index.html").Execute(w, snippet{
		Def: "here is def",
	})
}
