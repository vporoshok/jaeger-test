package amqpService

import (
	"context"
	"log"

	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/streadway/amqp"
)

type server struct {
	stop chan struct{}
}

func (s server) Run() {
	defer close(s.stop)

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

	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		log.Fatalf("ERROR: %s", err)
	}
	if err := ch.QueueBind(q.Name, "", "test", false, nil); err != nil {
		log.Fatalf("ERROR: %s", err)
	}

	msgs, err := ch.Consume(q.Name, "amqpService", false, true, false, false, nil)
	if err != nil {
		log.Fatalf("ERROR: %s", err)
	}

	for {
		select {
		case msg := <-msgs:
			log.Print(msg)
			headers := map[string]string{}
			for k, v := range msg.Headers {
				if s, ok := v.(string); ok {
					headers[k] = s
				}
			}
			wireContext, err := opentracing.GlobalTracer().Extract(
				opentracing.TextMap,
				opentracing.TextMapCarrier(headers))
			if err != nil {
				log.Printf("ERROR: %s", err)
			}

			sp := opentracing.StartSpan(
				"amqpService",
				ext.RPCServerOption(wireContext))

			time.Sleep(200 * time.Millisecond)

			msg.Ack(false)
			sp.Finish()
		case <-s.stop:
			return
		}
	}
}

// Shutdown service graceful
func (s server) Shutdown(_ context.Context) error {
	select {
	case <-s.stop:
	default:
		s.stop <- struct{}{}
		<-s.stop
	}

	return nil
}

// Run amqpService
func Run() *server {
	s := &server{
		stop: make(chan struct{}),
	}

	s.Run()

	return s
}
