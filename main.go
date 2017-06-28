package main

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

var (
	tmpl *template.Template

	conf, def string
)

func main() {
	if err := load(); err != nil {
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

func load() error {
	var err error
	tmpl, err = template.ParseFiles("index.html")
	if err != nil {
		return err
	}
	if conf, err = readFile(filepath.Join("default", "conf.json")); err != nil {
		return err
	}
	if def, err = readFile(filepath.Join("default", "def.json")); err != nil {
		return err
	}
	return nil
}

func readFile(path string) (string, error) {
	bs, err := ioutil.ReadFile(path)
	return string(bs), err
}

type snippet struct {
	Conf, Def string
}

func handler(w http.ResponseWriter, r *http.Request) {
	tmpl.Lookup("index.html").Execute(w, snippet{
		Conf: conf,
		Def:  def,
	})
}
