package httpService

import (
	"log"
	"net/http"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/streadway/amqp"
)

// Run http service on port 3001
func Run() *http.Server {
	s := &http.Server{
		Addr: ":3001",
	}
	go func() {
		cn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
		if err != nil {
			log.Fatalf("ERROR: %s", err)
		}
		defer cn.Close()

		ch, err := cn.Channel()
		if err != nil {
			log.Fatalf("ERROR: %s", err)
		}
		defer ch.Close()

		if err := ch.Confirm(false); err != nil {
			log.Fatalf("ERROR: %s", err)
		}

		if err := ch.ExchangeDeclare("test", "fanout", false, false, false, false, nil); err != nil {
			log.Fatalf("ERROR: %s", err)
		}

		m := http.NewServeMux()
		m.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
			var sp opentracing.Span
			wireContext, err := opentracing.GlobalTracer().Extract(
				opentracing.HTTPHeaders,
				opentracing.HTTPHeadersCarrier(req.Header))
			if err != nil {
				log.Printf("ERROR: %s", err)
			}

			sp = opentracing.StartSpan(
				"httpService",
				ext.RPCServerOption(wireContext))

			defer sp.Finish()

			time.Sleep(200 * time.Millisecond)

			headers := map[string]string{}

			opentracing.GlobalTracer().Inject(
				sp.Context(),
				opentracing.TextMap,
				opentracing.TextMapCarrier(headers))

			t := amqp.Table{}
			for k, v := range headers {
				t[k] = v
			}

			msg := amqp.Publishing{
				Headers: t,
				Body:    []byte("test"),
			}

			log.Print(ch)

			if err := ch.Publish("test", "", false, false, msg); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			http.Error(w, "OK", http.StatusOK)
		})

		s.Handler = m
		s.ListenAndServe()
	}()

	return s
}
