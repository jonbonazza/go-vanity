package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	texttemplate "text/template"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
)

var indexTmpl = template.Must(template.New("").Parse(`
<html>
  <body>
    <ul>
      {{range $pkg := .}}
        <li><a href="{{$pkg.Name}}">{{$pkg.Name}}</a></li>
      {{end}}
    </ul>
  </body>
</html>
`))

var pkgTmpl = template.Must(template.New("").Parse(`
<html>
  <head>
    <meta name="go-import" content="{{.Domain}}/{{.Org}}/{{.Name}} git https://{{.Domain}}/{{.Org}}/{{.Name}}">
  </head>
</html>
`))

var (
	requests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pkg_requests_total",
			Help: "Number of requests",
		},
		[]string{"path"},
	)

	errors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pkg_errors_total",
			Help: "Number of errors",
		},
		[]string{"path"},
	)
)

func init() {
	prometheus.MustRegister(requests)
	prometheus.MustRegister(errors)
}

var (
	base             = os.Getenv("BASE")
	listen           = os.Getenv("LISTEN")
	prometheusListen = os.Getenv("PROMETHEUS")
)

type context struct {
	Domain string
	Org    string
	Name   string
}

func handler(w http.ResponseWriter, r *http.Request) {
	requests.WithLabelValues(r.URL.Path).Inc()
	if r.URL.Path == "/" {
		return
	}
	vars := mux.Vars(r)
	ctx := context{
		Domain: base,
		Org:    vars["org"],
		Name:   vars["name"],
	}
	if err, ok := pkgTmpl.Execute(w, ctx).(texttemplate.ExecError); ok {
		errors.WithLabelValues(r.URL.Path).Inc()
		log.Println("error executing package template:", err)
	}
}

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintf(os.Stderr, `Usage: %s

Environment variables:

  LISTEN: On which host and port to serve the vanity imports. Default: ":8080"
  PKGBASE: The base of the vanity import paths. Example: "honnef.co/go"
  PKGFILE: The path to a JSON file describing all known packages
  PROMETHEUS: On which host and port to serve Prometheus metrics. The
    empty string disables Prometheus. Default: ""
`, os.Args[0])
		os.Exit(1)
	}
	if listen == "" {
		listen = ":8080"
	}
	if prometheusListen != "" {
		go func() {
			if err := http.ListenAndServe(prometheusListen, prometheus.Handler()); err != nil {
				log.Fatal(err)
			}
		}()
	}
	r := mux.NewRouter()
	r.HandleFunc("/{org}/{name}", handler).Methods(http.MethodGet)
	if err := http.ListenAndServe(listen, r); err != nil {
		log.Fatal(err)
	}
}
