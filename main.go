package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/buger/jsonparser"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

var (
	tmpl *template.Template

	defConf, defDef string

	cmdMu     sync.Mutex
	cmd       *exec.Cmd
	cmdCancel context.CancelFunc

	baseURL string

	listen   = flag.String("l", ":8081", "address to listen on")
	gwURLStr = flag.String("gw", "http://localhost:8080", "local gateway URL")
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
	gwURL, err := url.Parse(*gwURLStr)
	if err != nil {
		log.Fatal(err)
	}
	revProxy := httputil.NewSingleHostReverseProxy(gwURL)
	r.Get("/gw/*", http.StripPrefix("/gw", revProxy).ServeHTTP)
	baseURL = *listen
	if strings.HasPrefix(baseURL, ":") {
		baseURL = "http://localhost" + baseURL
	}
	fmt.Printf("Open %s in your browser\n", baseURL)
	http.ListenAndServe(*listen, r)
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

func restart(w http.ResponseWriter, r *http.Request) {
	conf := []byte(r.FormValue("conf"))
	def := []byte(r.FormValue("def"))
	if !json.Valid(conf) || !json.Valid(def) {
		http.Error(w, "invalid JSON", 400)
		return
	}
	listenPath, _ := jsonparser.GetString(def, "proxy", "listen_path")
	if listenPath == "" {
		http.Error(w, "empty listen_path", 400)
		return
	}
	if err := restartCmd(r); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	io.WriteString(w, baseURL+"/gw"+listenPath)
}

type tmplBody struct {
	BaseURL   string
	Conf, Def string
}

func index(w http.ResponseWriter, r *http.Request) {
	if err := restartCmd(r); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	tmpl.Lookup("index.html").Execute(w, tmplBody{
		BaseURL: baseURL,
		Conf:    defConf,
		Def:     defDef,
	})
}
