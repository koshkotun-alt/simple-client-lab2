package main

import (
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"simple-web-server/src/client"
	"simple-web-server/src/tracer"
	"sync"
)

type TemplateHandler struct {
	once     sync.Once
	template *template.Template
	filename string
}

func (t *TemplateHandler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	t.once.Do(func() {
		t.template = template.Must(template.ParseFiles(filepath.Join("src", "templates", t.filename)))
	})

	t.template.Execute(writer, req)
}

func main() {
	var addr = flag.String("addr", ":8080", "The addr of the app")
	flag.Parse()

	r := client.NewRoom()
	r.Tracer = tracer.New(os.Stdout)

	http.Handle("/", &TemplateHandler{filename: "index.html"})
	http.Handle("/room", r)

	go r.Run()

	if err := http.ListenAndServe(*addr, nil); err != nil {
		fmt.Fprintf(os.Stderr, "ListenAndServe: %v\n", err)
	}
}
