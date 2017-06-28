package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

var (
	tmpl *template.Template

	conf, def map[string]interface{}
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
	if conf, err = parseFile(filepath.Join("default", "conf.json")); err != nil {
		return err
	}
	if def, err = parseFile(filepath.Join("default", "def.json")); err != nil {
		return err
	}
	return nil
}

func parseFile(path string) (map[string]interface{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	err = json.NewDecoder(f).Decode(&m)
	return m, err
}

type snippet struct {
	Conf, Def string
}

func handler(w http.ResponseWriter, r *http.Request) {
	jsonConf, _ := json.MarshalIndent(conf, "", "\t")
	jsonDef, _ := json.MarshalIndent(def, "", "\t")
	tmpl.Lookup("index.html").Execute(w, snippet{
		Conf: string(jsonConf),
		Def:  string(jsonDef),
	})
}
