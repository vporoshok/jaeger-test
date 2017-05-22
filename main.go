package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/vporoshok/jaeger-test/httpService"
	"golang.org/x/net/context"

	jaegerClientConfig "github.com/uber/jaeger-client-go/config"
)

// Run http service on port 8080
func Run() *http.Server {
	m := http.NewServeMux()
	m.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		sp := opentracing.StartSpan("main")

		defer sp.Finish()

		time.Sleep(200)

		httpClient := &http.Client{}
		httpReq, _ := http.NewRequest("GET", "http://localhost:3001/", nil)

		// Transmit the span's TraceContext as HTTP headers on our
		// outbound request.
		opentracing.GlobalTracer().Inject(
			sp.Context(),
			opentracing.HTTPHeaders,
			opentracing.HTTPHeadersCarrier(httpReq.Header))

		resp, err := httpClient.Do(httpReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		body := &bytes.Buffer{}

		io.Copy(body, resp.Body)

		http.Error(w, body.String(), resp.StatusCode)
	})

	s := &http.Server{
		Addr:    ":8080",
		Handler: m,
	}
	go s.ListenAndServe()

	return s
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Print("Init trancer")
	jcfg := jaegerClientConfig.Configuration{
		Reporter: &jaegerClientConfig.ReporterConfig{
			LocalAgentHostPort: "jaeger:5775",
		},
		Sampler: &jaegerClientConfig.SamplerConfig{
			Type:  "const",
			Param: 1.0, // sample all traces
		},
	}
	if closer, err := jcfg.InitGlobalTracer("test"); err != nil {
		log.Printf("ERROR: %s", err)
	} else {
		defer closer.Close()
	}

	log.Print("Run")
	servers := map[string]interface{}{
		"main": Run(),
		"http": httpService.Run(),
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c
	log.Print("Stoping...")
	wg := sync.WaitGroup{}
	for name, server := range servers {
		wg.Add(1)
		go func(name string, server interface{}) {
			switch s := server.(type) {
			case *http.Server:
				s.Shutdown(context.Background())
			default:
				log.Printf("Unknown server type: %#v", server)
			}
			log.Printf("%s is stopped", name)
			wg.Done()
		}(name, server)
	}
	wg.Wait()
}
