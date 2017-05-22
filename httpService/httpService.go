package httpService

import (
	"log"
	"net/http"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

// Run http service on port 3001
func Run() *http.Server {
	m := http.NewServeMux()
	m.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		var serverSpan opentracing.Span
		wireContext, err := opentracing.GlobalTracer().Extract(
			opentracing.HTTPHeaders,
			opentracing.HTTPHeadersCarrier(req.Header))
		if err != nil {
			log.Printf("ERROR: %s", err)
		}

		serverSpan = opentracing.StartSpan(
			"httpService",
			ext.RPCServerOption(wireContext))

		defer serverSpan.Finish()

		time.Sleep(200)

		http.Error(w, "OK", http.StatusOK)
	})

	s := &http.Server{
		Addr:    ":3001",
		Handler: m,
	}
	go s.ListenAndServe()

	return s
}
