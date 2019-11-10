package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (dd *DarknetD) startAPI(addr string) error {
	r := mux.NewRouter()
	r.HandleFunc("/", httpRootHandler).Methods("GET")
	r.HandleFunc("/objects", dd.httpObjectsHandler).Methods("GET")
	r.HandleFunc("/latest.jpg", dd.httpLatestHandler).Methods("GET")
	r.HandleFunc("/image/{imgname}", dd.httpImageHandler).Methods("GET")

	registerMetricsHandlers(r)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}
	return srv.ListenAndServe()
}

const rootHtml = `<html><body>
<h1>darknetd API</h1>
<ul>
<li> <a href="objects">/objects</a>: returns JSON list of most recent predictions
<li> <a href="latest.jpg">/latest.jpg</a>: returns latest source image
<li> /image/{imagename}.jpg: returns source or prediction image (get {imagename} from /objects output)
<li> <a href="metrics">/metrics</a>: returns performance metrics in prometheus format
<li> <a href="health">/health</a>: returns 'OK' if healthy
</ul>
</body></html>`

func httpRootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, rootHtml)
}

func (dd *DarknetD) httpObjectsHandler(w http.ResponseWriter, r *http.Request) {
	out, err := json.Marshal(dd.detections.Values())
	if err != nil {
		e := fmt.Errorf("Result processing error: %s", err)
		fmt.Println(e)
		http.Error(w, e.Error(), http.StatusInternalServerError)
		dd.metrics.ApiErrors.WithLabelValues("/objects", "json.Marshal").Add(1)
		return
	}
	fmt.Fprintln(w, string(out))
	dd.metrics.ApiRequests.WithLabelValues("/objects").Add(1)
}

func (dd *DarknetD) httpImageHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Handling image request")
	vars := mux.Vars(r)
	imgName, exist := vars["imgname"]
	if !exist {
		e := fmt.Errorf("Image name missing")
		fmt.Println(e)
		http.Error(w, e.Error(), http.StatusNotFound)
		dd.metrics.ApiErrors.WithLabelValues("/image/", "NameMissing").Add(1)
		return
	}
	if !strings.HasSuffix(imgName, "jpg") {
		e := fmt.Errorf("Invalid image request: %s", imgName)
		fmt.Println(e)
		http.Error(w, e.Error(), http.StatusNotFound)
		dd.metrics.ApiErrors.WithLabelValues("/image/", "NotJPG").Add(1)
		return
	}
	imgFile := filepath.Join(dd.config.archiveDir, imgName)
	f, err := os.Open(imgFile)
	if err != nil {
		e := fmt.Errorf("Error accessing image at %s: %s", imgFile, err)
		fmt.Println(e)
		http.Error(w, e.Error(), http.StatusNotFound)
		dd.metrics.ApiErrors.WithLabelValues("/image/", "ImageOpen").Add(1)
		return
	}
	w.Header().Set("Content-Type", "image/jpg")
	_, err = io.Copy(w, f)
	if err != nil {
		e := fmt.Errorf("Error reading image at %s: %s", imgFile, err)
		fmt.Println(e)
		http.Error(w, e.Error(), http.StatusInternalServerError)
		dd.metrics.ApiErrors.WithLabelValues("/image/", "ImageCopy").Add(1)
		return
	}
	log.Printf("Handled image request for %s", imgName)
	dd.metrics.ApiRequests.WithLabelValues("/image/").Add(1)
}

func (dd *DarknetD) httpLatestHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Handling latest image request")
	imgFile := filepath.Join(dd.config.capDir, dd.config.capFile)
	f, err := os.Open(imgFile)
	if err != nil {
		e := fmt.Errorf("Error accessing latest image at %s: %s", imgFile, err)
		fmt.Println(e)
		http.Error(w, e.Error(), http.StatusNotFound)
		dd.metrics.ApiErrors.WithLabelValues("/latest.jpg", "ImageOpen").Add(1)
		return
	}
	w.Header().Set("Content-Type", "image/jpg")
	_, err = io.Copy(w, f)
	if err != nil {
		e := fmt.Errorf("Error reading latest image at %s: %s", imgFile, err)
		fmt.Println(e)
		http.Error(w, e.Error(), http.StatusInternalServerError)
		dd.metrics.ApiErrors.WithLabelValues("/latest.jpg", "ImageCopy").Add(1)
		return
	}
	log.Printf("Handled latest image request")
	dd.metrics.ApiRequests.WithLabelValues("/latest.jpg").Add(1)
}

func registerMetricsHandlers(r *mux.Router) {
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "OK")
	})
	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/debug/pprof/", pprof.Index)
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/trace", pprof.Trace)
	r.Handle("/debug/pprof/block", pprof.Handler("block"))
	r.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	r.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	r.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
}
