package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Llongfile)
}

var (
	// nats://...
	// ws://...
	// tls://...
	servers = []string{
		"nats://127.0.0.1:4222",
		"nats://127.0.0.1:5222",
		"nats://127.0.0.1:6222",
	}
	User = "root1"
	//cmd:nats server passwd
	Pass = "1234567890098765432112"
)

//https://natsbyexample.com/examples/jetstream/
func main() {
	//cmd :`nats stream rm EVENTS`
	limits_stream()

	//cmd :`nats stream rm EVENTS`
	interest_stream()

	//cmd :`nats stream rm EVENTS`
	workqueue_stream()

	QueuePubSub()

	PubSub()
}

// Connect to NATS
func createNc() (nc *nats.Conn, deferFunc func()) {
	var err error
	nc, err = nats.Connect(strings.Join(servers, ","),
		nats.UserInfo(User, Pass),
		// nats.MaxReconnects(60),
		// nats.NoReconnect(),
		// nats.DontRandomize(),
		// nats.ReconnectWait(10*time.Second),
		// nats.ReconnectBufSize(8*1024*1024),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			log.Printf("client disconnected: %v\n", err)
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			log.Println("client reconnected")
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			log.Println("client closed")
		}),
		nats.DiscoveredServersHandler(func(nc *nats.Conn) {
			log.Printf("Known servers: %v\n", nc.Servers())
			log.Printf("Discovered servers: %v\n", nc.DiscoveredServers())
		}),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			log.Printf("Error: %v\n", err)
		}))
	if err != nil {
		log.Fatal(err)
		return
	}
	log.Printf("The connection is %v\n", nc.Status().String())
	deferFunc = func() {
		nc.Drain()
		log.Printf("The connection is %v\n", nc.Status().String())
	}
	return
}

// Create JetStream Context
func createJs() (js nats.JetStreamContext, deferFunc func()) {
	var nc *nats.Conn
	nc, deferFunc = createNc()
	var err error
	js, err = nc.JetStream()
	if err != nil {
		log.Fatal(err)
	}
	return
}

func printStreamState(js nats.JetStreamContext, name string) {
	info, err := js.StreamInfo(name)
	if err != nil {
		log.Println(err)
		return
	}
	b, _ := json.MarshalIndent(info.State, "", " ")
	log.Println("inspection stream info")
	log.Println(string(b))
}

func QueuePubSub() {
	log.Println("limits_stream\n")
	nc, cancel := createNc()
	defer cancel()

	nc.QueueSubscribe("foo", "queue.foo", func(msg *nats.Msg) {
		log.Println("Subscribe 1:", string(msg.Data))
	})

	nc.QueueSubscribe("foo", "queue.foo", func(msg *nats.Msg) {
		log.Println("Subscribe 2:", string(msg.Data))
	})

	nc.QueueSubscribe("foo", "queue.foo", func(msg *nats.Msg) {
		log.Println("Subscribe 3:", string(msg.Data))
	})

	for i := 1; i <= 3; i++ {
		message := fmt.Sprintf("Message %d", i)

		if err := nc.Publish("foo", []byte(message)); err != nil {
			log.Fatal(err)
		}
	}

	time.Sleep(2 * time.Second)
}

func PubSub() {
	log.Println("limits_stream\n")
	nc, cancel := createNc()
	defer cancel()

	g := sync.WaitGroup{}
	g.Add(3)
	go func() {
		defer g.Done()
		// Use a WaitGroup to wait for 2 messages to arrive
		wg := sync.WaitGroup{}
		wg.Add(2)

		// Subscribe
		if _, err := nc.Subscribe("time.*.east", func(m *nats.Msg) {
			log.Printf("%s: %s\n", m.Subject, m.Data)
			wg.Done()
		}); err != nil {
			log.Fatal(err)
		}

		// Wait for the 2 messages to come in
		wg.Wait()
	}()

	go func() {
		defer g.Done()
		// Use a WaitGroup to wait for 4 messages to arrive
		wg := sync.WaitGroup{}
		wg.Add(4)

		// Subscribe
		if _, err := nc.Subscribe("time.>", func(m *nats.Msg) {
			log.Printf("%s: %s\n", m.Subject, m.Data)
			wg.Done()
		}); err != nil {
			log.Fatal(err)
		}

		// Wait for the 4 messages to come in
		wg.Wait()

		// Close the connection
		nc.Close()
	}()

	go func() {
		defer g.Done()
		zoneID, err := time.LoadLocation("America/New_York")
		if err != nil {
			log.Fatal(err)
		}
		now := time.Now()
		zoneDateTime := now.In(zoneID)
		formatted := zoneDateTime.String()

		nc.Publish("time.us.east", []byte(formatted))
		nc.Publish("time.us.east.atlanta", []byte(formatted))

		zoneID, err = time.LoadLocation("Europe/Warsaw")
		if err != nil {
			log.Fatal(err)
		}
		zoneDateTime = now.In(zoneID)
		formatted = zoneDateTime.String()

		nc.Publish("time.eu.east", []byte(formatted))
		nc.Publish("time.eu.east.warsaw", []byte(formatted))
	}()
	g.Wait()
}

//Retention: nats.InterestPolicy https://natsbyexample.com/examples/jetstream/interest-stream/go
func interest_stream() {
	log.Println("limits_stream\n")
	js, cancel := createJs()
	defer cancel()
	cfg := &nats.StreamConfig{
		Name:      "EVENTS",
		Retention: nats.InterestPolicy,
		Subjects:  []string{"events.>"},
	}

	js.AddStream(cfg)
	defer js.DeleteStream(cfg.Name)
	log.Println("created the stream")

	js.Publish("events.page_loaded", nil)
	js.Publish("events.mouse_clicked", nil)
	ack, _ := js.Publish("events.input_focused", nil)
	log.Println("published 3 messages")

	log.Printf("last message seq: %d\n", ack.Sequence)

	log.Println("# Stream info without any consumers")
	printStreamState(js, cfg.Name)

	js.AddConsumer(cfg.Name, &nats.ConsumerConfig{
		Durable:   "processor-1",
		AckPolicy: nats.AckExplicitPolicy,
	})

	js.Publish("events.mouse_clicked", nil)
	js.Publish("events.input_focused", nil)

	log.Println("# Stream info with one consumer")
	printStreamState(js, cfg.Name)

	sub1, _ := js.PullSubscribe("", "processor-1", nats.Bind(cfg.Name, "processor-1"))
	defer sub1.Unsubscribe()

	msgs, _ := sub1.Fetch(2)
	msgs[0].Ack()
	msgs[1].AckSync()

	log.Println("# Stream info with one consumer and acked messages")
	printStreamState(js, cfg.Name)

	js.AddConsumer(cfg.Name, &nats.ConsumerConfig{
		Durable:   "processor-2",
		AckPolicy: nats.AckExplicitPolicy,
	})

	js.Publish("events.input_focused", nil)
	js.Publish("events.mouse_clicked", nil)

	sub2, _ := js.PullSubscribe("", "processor-2", nats.Bind(cfg.Name, "processor-2"))
	defer sub2.Unsubscribe()

	msgs, _ = sub2.Fetch(2)
	md0, _ := msgs[0].Metadata()
	md1, _ := msgs[1].Metadata()
	log.Printf("msg seqs %d and %d\n", md0.Sequence.Stream, md1.Sequence.Stream)
	msgs[0].Ack()
	msgs[1].AckSync()

	log.Println("# Stream info with two consumers, but only one set of acked messages")
	printStreamState(js, cfg.Name)

	msgs, _ = sub1.Fetch(2)
	msgs[0].Ack()
	msgs[1].AckSync()

	log.Println("# Stream info with two consumers having both acked")
	printStreamState(js, cfg.Name)

	js.AddConsumer(cfg.Name, &nats.ConsumerConfig{
		Durable:       "processor-3",
		AckPolicy:     nats.AckExplicitPolicy,
		FilterSubject: "events.mouse_clicked",
	})

	js.Publish("events.input_focused", nil)

	msgs, _ = sub1.Fetch(1)
	msgs[0].Term()
	msgs, _ = sub2.Fetch(1)
	msgs[0].AckSync()

	log.Println("# Stream info with three consumers with interest from two")
	printStreamState(js, cfg.Name)
}

//Retention: nats.WorkQueuePolicy https://natsbyexample.com/examples/jetstream/workqueue-stream/go
func workqueue_stream() {
	log.Println("limits_stream\n")
	js, cancel := createJs()
	defer cancel()
	cfg := &nats.StreamConfig{
		Name:      "EVENTS",
		Retention: nats.WorkQueuePolicy,
		Subjects:  []string{"events.>"},
	}

	js.AddStream(cfg)
	defer js.DeleteStream(cfg.Name)
	log.Println("created the stream")

	// js.Publish("events.us.page_loaded", nil)
	// js.Publish("events.eu.mouse_clicked", nil)
	// js.Publish("events.us.input_focused", nil)
	// log.Println("published 3 messages")

	log.Println("# Stream info without any consumers")
	printStreamState(js, cfg.Name)

	sub1, _ := js.PullSubscribe("", "processor-1", nats.BindStream(cfg.Name))

	msgs, _ := sub1.Fetch(3)
	for _, msg := range msgs {
		log.Printf("sub got: %s\n", msg.Subject)
		msg.AckSync()
	}

	log.Println("# Stream info with one consumer")
	printStreamState(js, cfg.Name)

	_, err := js.PullSubscribe("", "processor-2", nats.BindStream(cfg.Name))
	log.Println("# Create an overlapping consumer")
	log.Println(err)
	printStreamState(js, cfg.Name)

	log.Println("Unsubscribe one consumer")
	sub1.Unsubscribe()
	printStreamState(js, cfg.Name)

	sub2, err := js.PullSubscribe("", "processor-2", nats.BindStream(cfg.Name))
	log.Printf("created the new consumer? %v\n", err == nil)
	printStreamState(js, cfg.Name)
	sub2.Unsubscribe()
	log.Println("Unsubscribe new consumer")
	printStreamState(js, cfg.Name)

	log.Println("# Create non-overlapping consumers")
	sub1, _ = js.PullSubscribe("events.us.>", "processor-us", nats.BindStream(cfg.Name))
	sub2, _ = js.PullSubscribe("events.eu.>", "processor-eu", nats.BindStream(cfg.Name))

	js.Publish("events.eu.mouse_clicked", nil)
	js.Publish("events.us.page_loaded", nil)
	js.Publish("events.us.input_focused", nil)
	js.Publish("events.eu.page_loaded", nil)
	log.Println("published 4 messages")

	msgs, _ = sub1.Fetch(2)
	for _, msg := range msgs {
		log.Printf("us sub got: %s\n", msg.Subject)
		msg.Ack()
	}

	msgs, _ = sub2.Fetch(2)
	for _, msg := range msgs {
		log.Printf("eu sub got: %s\n", msg.Subject)
		msg.Ack()
	}
	printStreamState(js, cfg.Name)
}

//Retention: nats.LimitsPolicy https://natsbyexample.com/examples/jetstream/limits-stream/go
func limits_stream() {
	log.Println("limits_stream\n")
	js, cancel := createJs()
	defer cancel()

	cfg := nats.StreamConfig{
		Name:      "EVENTS",
		Retention: nats.LimitsPolicy,
		Subjects:  []string{"events.>"},
	}

	cfg.Storage = nats.FileStorage

	js.AddStream(&cfg)
	defer js.DeleteStream(cfg.Name)
	log.Println("created the stream")

	js.Publish("events.page_loaded", nil)
	js.Publish("events.mouse_clicked", nil)
	js.Publish("events.mouse_clicked", nil)
	js.Publish("events.page_loaded", nil)
	js.Publish("events.mouse_clicked", nil)
	js.Publish("events.input_focused", nil)
	log.Println("published 6 messages")

	js.PublishAsync("events.input_changed", nil)
	js.PublishAsync("events.input_blurred", nil)
	js.PublishAsync("events.key_pressed", nil)
	js.PublishAsync("events.input_focused", nil)
	js.PublishAsync("events.input_changed", nil)
	js.PublishAsync("events.input_blurred", nil)

	select {
	case <-js.PublishAsyncComplete():
		log.Println("published 6 messages")
	case <-time.After(time.Second):
		log.Fatal("publish took too long")
	}

	printStreamState(js, cfg.Name)

	cfg.MaxMsgs = 10
	js.UpdateStream(&cfg)
	log.Println("set max messages to 10")

	printStreamState(js, cfg.Name)

	cfg.MaxBytes = 300
	js.UpdateStream(&cfg)
	log.Println("set max bytes to 300")

	printStreamState(js, cfg.Name)

	cfg.MaxAge = time.Second
	js.UpdateStream(&cfg)
	log.Println("set max age to one second")

	printStreamState(js, cfg.Name)

	log.Println("sleeping one second...")
	time.Sleep(time.Second)

	printStreamState(js, cfg.Name)
}
