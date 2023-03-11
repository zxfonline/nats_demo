package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Llongfile)
}

var (
	URL  = "nats://localhost:4222,nats://localhost:5222,nats://localhost:6222"
	User = "root1"
	//cmd:nats server passwd
	Pass = "1234567890098765432112"
)

func createJs() (js nats.JetStreamContext, deferFunc func()) {
	// Connect to NATS
	nc, err := nats.Connect(URL, nats.UserInfo(User, Pass))
	if err != nil {
		log.Fatal(err)
	}

	// Create JetStream Context
	js, err = nc.JetStream()
	if err != nil {
		log.Fatal(err)
	}
	return js, func() { nc.Drain() }
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

func main1() {
	nc, err := nats.Connect(URL, nats.UserInfo(User, Pass))
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

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

func main2() {
	nc, err := nats.Connect(URL, nats.UserInfo(User, Pass))
	if err != nil {
		log.Fatal(err)
	}

	// Create JetStream Context
	js, err := nc.JetStream()
	if err != nil {
		log.Fatal(err)
	}
	// Create a Stream
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "ORDERS",
		Subjects: []string{"ORDERS.*"},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Update a Stream
	_, err = js.UpdateStream(&nats.StreamConfig{
		Name:     "ORDERS",
		MaxBytes: 8,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Create a Consumer
	_, err = js.AddConsumer("ORDERS", &nats.ConsumerConfig{
		Durable: "MONITOR",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Delete Consumer
	err = js.DeleteConsumer("ORDERS", "MONITOR")

	if err != nil {
		log.Fatal(err)
	}
	// Delete Stream
	err = js.DeleteStream("ORDERS")
	if err != nil {
		log.Fatal(err)
	}
}

//https://natsbyexample.com/examples/jetstream/
func main() {
	////cmd :`nats stream rm EVENTS`
	// limits_stream()

	////cmd :`nats stream rm EVENTS`
	interest_stream()

	////cmd :`nats stream rm EVENTS`
	// workqueue_stream()
}

//Retention: nats.InterestPolicy https://natsbyexample.com/examples/jetstream/interest-stream/go
func interest_stream() {
	js, cancel := createJs()
	defer cancel()
	cfg := &nats.StreamConfig{
		Name:      "EVENTS",
		Retention: nats.InterestPolicy,
		Subjects:  []string{"events.>"},
	}

	js.AddStream(cfg)
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
	js, cancel := createJs()
	defer cancel()
	cfg := &nats.StreamConfig{
		Name:      "EVENTS",
		Retention: nats.WorkQueuePolicy,
		Subjects:  []string{"events.>"},
	}

	js.AddStream(cfg)
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
	js, cancel := createJs()
	defer cancel()

	cfg := nats.StreamConfig{
		Name:      "EVENTS",
		Retention: nats.LimitsPolicy,
		Subjects:  []string{"events.>"},
	}

	cfg.Storage = nats.FileStorage

	js.AddStream(&cfg)
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
