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
	r.Get("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))).ServeHTTP)
	r.Get("/", handler)
	http.ListenAndServe(":8080", r)
}

type snippet struct {
	Conf, Def string
}

func handler(w http.ResponseWriter, r *http.Request) {
	tmpl.Lookup("index.html").Execute(w, snippet{
		Conf: "here is conf",
		Def:  "here is def",
	})
}
