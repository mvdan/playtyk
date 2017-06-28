package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
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
	cmdBuf    *bytes.Buffer

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
	r.Post("/share", share)
	r.Get("/output", output)
	r.Get("/", index)
	gwURL, err := url.Parse(*gwURLStr)
	if err != nil {
		log.Fatal(err)
	}
	revProxy := httputil.NewSingleHostReverseProxy(gwURL)
	r.HandleFunc("/gw/tyk/*", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "no!")
	})
	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("static", "favicon.ico"))
	})
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
	cmdBuf = new(bytes.Buffer)
	cmd.Stdout = cmdBuf
	cmd.Stderr = cmdBuf
	cmd.Dir = "gateway"
	cmd.Env = os.Environ()
	return cmd.Start()
}

func readFile(path string) (string, error) {
	bs, err := ioutil.ReadFile(path)
	return string(bs), err
}

func writeFile(path, data string) error {
	return ioutil.WriteFile(path, []byte(data), 0644)
}

func pairFromForm(r *http.Request) (string, string, error) {
	conf := r.FormValue("conf")
	if !json.Valid([]byte(conf)) {
		return "", "", fmt.Errorf("the gateway config is not valid JSON")
	}
	def := r.FormValue("def")
	if !json.Valid([]byte(def)) {
		return "", "", fmt.Errorf("the API definition is not valid JSON")
	}
	return conf, def, nil
}

func restart(w http.ResponseWriter, r *http.Request) {
	_, def, err := pairFromForm(r)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	listenPath, _ := jsonparser.GetString([]byte(def), "proxy", "listen_path")
	if listenPath == "" {
		http.Error(w, "empty or missing listen_path", 400)
		return
	}
	if err := restartCmd(r); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	io.WriteString(w, baseURL+"/gw"+listenPath)
}

func base64SHA1(bs []byte) string {
	hasher := sha1.New()
	hasher.Write(bs)
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func share(w http.ResponseWriter, r *http.Request) {
	conf, def, err := pairFromForm(r)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err := restartCmd(r); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	state := pageState{Conf: conf, Def: def}
	enc, err := json.Marshal(state)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	name := base64SHA1(enc)[:10]
	io.WriteString(w, baseURL+"/s/"+name)
}

func output(w http.ResponseWriter, r *http.Request) {
	cmdMu.Lock()
	defer cmdMu.Unlock()
	io.WriteString(w, cmdBuf.String())
}

type pageState struct {
	BaseURL   string
	Conf, Def string
}

func index(w http.ResponseWriter, r *http.Request) {
	if err := restartCmd(r); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	tmpl.Lookup("index.html").Execute(w, pageState{
		BaseURL: baseURL,
		Conf:    defConf,
		Def:     defDef,
	})
}
