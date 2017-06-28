package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

var (
	tmpl *template.Template

	defConf, defDef string

	cmdMu     sync.Mutex
	cmd       *exec.Cmd
	cmdCancel context.CancelFunc
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
	r.Post("/restart", restart)
	r.Get("/", index)
	gwURL, err := url.Parse("http://localhost:8080")
	if err != nil {
		log.Fatal(err)
	}
	revProxy := httputil.NewSingleHostReverseProxy(gwURL)
	r.Get("/gw/*", http.StripPrefix("/gw", revProxy).ServeHTTP)
	fmt.Println("Serving on http://localhost:8081")
	http.ListenAndServe(":8081", r)
}

func load() error {
	var err error
	tmpl, err = template.ParseFiles("index.html")
	if err != nil {
		return err
	}
	if defConf, err = readFile(filepath.Join("default", "conf.json")); err != nil {
		return err
	}
	if defDef, err = readFile(filepath.Join("default", "def.json")); err != nil {
		return err
	}
	return nil
}

func restartCmd(r *http.Request) error {
	cmdMu.Lock()
	defer cmdMu.Unlock()
	if cmd != nil {
		cmdCancel()
	}
	ctx, fn := context.WithCancel(context.Background())
	cmdCancel = fn
	conf, def := defConf, defDef
	if r.Method == "POST" {
		conf = r.FormValue("conf")
		def = r.FormValue("def")
	}
	if err := writeFile(filepath.Join("gateway", "conf.json"), conf); err != nil {
		return err
	}
	if err := writeFile(filepath.Join("gateway", "apps", "test.json"), def); err != nil {
		return err
	}
	cmd = exec.CommandContext(ctx, "tyk", "--conf=conf.json")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = "gateway"
	return cmd.Start()
}

func readFile(path string) (string, error) {
	bs, err := ioutil.ReadFile(path)
	return string(bs), err
}

func writeFile(path, data string) error {
	return ioutil.WriteFile(path, []byte(data), 0644)
}

type snippet struct {
	Conf, Def string
}

func restart(w http.ResponseWriter, r *http.Request) {
	var def map[string]interface{}
	if err := json.Unmarshal([]byte(r.FormValue("def")), &def); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	//listenPath := def["proxy"].(map[string]interface{})["listen_path"].(string)
	if err := restartCmd(r); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	if err := restartCmd(r); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	tmpl.Lookup("index.html").Execute(w, snippet{
		Conf: defConf,
		Def:  defDef,
	})
}
