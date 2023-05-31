package main

import (
	"encoding/json"
	"errors"
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
		"nats://127.0.0.1:4222,nats://127.0.0.1:4223,nats://127.0.0.1:4224",
		// "nats://127.0.0.1:4322,nats://127.0.0.1:4323,nats://127.0.0.1:4324",
	}
	User = "app"
	//cmd:nats server passwd
	Pass = "app"
)

const (
	StreamName_EVENTS     = "EVENTS"
	StreamSubjects_EVENTS = "events.>"
)

// https://natsbyexample.com/examples/jetstream/
func main() {
	func() {
		nc, js, cancel := createJs()
		defer cancel()
		// limits_stream(nc, js)
		// interest_stream(nc, js)
		workqueue_stream(nc, js)
		// pull_consumer(nc, js)
		// pull_consumer_limits(nc, js)
		// push_consumer(nc, js)
		// queue_push_consumer(nc, js)
		// multi_stream_consumption(nc, js)
	}()

	// QueuePubSub()
	// PubSub()
	// DrainDemo()

	// EncodeDemo()
}

// Connect to NATS
func createNc() (nc *nats.Conn, deferFunc func()) {
	var err error
	nc, err = nats.Connect(strings.Join(servers, ","),
		nats.UserInfo(User, Pass),
		nats.Timeout(10*time.Second),
		nats.PingInterval(10*time.Second),
		nats.MaxPingsOutstanding(3),
		// nats.MaxReconnects(60),
		// nats.NoReconnect(),
		// nats.DontRandomize(),
		// nats.ReconnectWait(10*time.Second),
		// nats.ReconnectBufSize(8*1024*1024),
		nats.DisconnectErrHandler(func(conn *nats.Conn, err error) {
			log.Printf("client disconnected err:%v,status:%v servers:%v\n", err, conn.Status().String(), conn.Servers())
		}),
		nats.DisconnectHandler(func(conn *nats.Conn) {
			log.Printf("client disconnected from %v addr:%v cluster:%v id:%v status:%v servers:%v\n", conn.ConnectedServerName(), conn.ConnectedAddr(), conn.ConnectedClusterName(), conn.ConnectedServerId(), conn.Status().String(), conn.Servers())
		}),
		nats.ConnectHandler(func(conn *nats.Conn) {
			// log.Printf("client connected to %+v\n", conn)
			log.Printf("client connected to %v addr:%v cluster:%v id:%v status:%v servers:%v\n", conn.ConnectedServerName(), conn.ConnectedAddr(), conn.ConnectedClusterName(), conn.ConnectedServerId(), conn.Status().String(), conn.Servers())
		}),
		nats.ReconnectHandler(func(conn *nats.Conn) {
			log.Printf("client reconnected to %v addr:%v cluster:%v id:%v status:%v servers:%v\n", conn.ConnectedServerName(), conn.ConnectedAddr(), conn.ConnectedClusterName(), conn.ConnectedServerId(), conn.Status().String(), conn.Servers())
		}),
		nats.ClosedHandler(func(conn *nats.Conn) {
			log.Printf("client connection closed,status:%v servers:%v\n", conn.Status().String(), conn.Servers())
		}),
		nats.DiscoveredServersHandler(func(conn *nats.Conn) {
			log.Printf("client discover servers %v addr:%v cluster:%v id:%v status:%v servers:%v discovered:%v\n", conn.ConnectedServerName(), conn.ConnectedAddr(), conn.ConnectedClusterName(), conn.ConnectedServerId(), conn.Status().String(), conn.Servers(), conn.DiscoveredServers())
		}),
		nats.ErrorHandler(func(conn *nats.Conn, s *nats.Subscription, err error) {
			if s != nil {
				log.Printf("client connection async err status:%v servers:%v in %q/%q,err:%v\n", conn.Status().String(), conn.Servers(), s.Subject, s.Queue, err)
			} else {
				log.Printf("client connection async err status:%v servers:%v err:%v\n", conn.Status().String(), conn.Servers(), err)
			}
		}))
	if err != nil {
		log.Fatalln(err)
		return
	}
	// log.Printf("The connection is %v\n", nc.Status().String())
	deferFunc = func() {
		nc.Drain()
		log.Printf("The connection is %v\n", nc.Status().String())
	}
	return
}

// Create JetStream Context
func createJs() (nc *nats.Conn, js nats.JetStreamContext, deferFunc func()) {
	nc, deferFunc = createNc()
	var err error
	js, err = nc.JetStream()
	if err != nil {
		deferFunc()
		log.Fatalln(err)
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
	log.Println("	inspection stream info")
	log.Println(string(b))
}

func EncodeDemo() {
	log.Println("EncodeDemo")
	nc, cancel := createNc()
	defer cancel()
	ec, err := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
	if err != nil {
		log.Panicln(err)
	}
	defer ec.Close()

	// Define the object
	type stock struct {
		Symbol string
		Price  int
	}

	wg := sync.WaitGroup{}
	wg.Add(2)

	// Subscribe
	// Decoding errors will be passed to the function supplied via
	// nats.ErrorHandler above, and the callback supplied here will
	// not be invoked.
	// Just to not collide using the demo server with other users.
	subject := nats.NewInbox()
	if _, err := ec.Subscribe(subject, func(s *stock) {
		log.Printf("Stock: %s - Price: %v", s.Symbol, s.Price)
		wg.Done()
	}); err != nil {
		log.Panicln(err)
	}
	cv, _ := json.Marshal(&stock{Symbol: "symbolV", Price: 110})
	if err := nc.Publish(subject, cv); err != nil {
		log.Panicln(err)
	}
	if err := ec.Publish(subject, &stock{Symbol: "GOOG", Price: 1200}); err != nil {
		log.Panicln(err)
	}
	// Sends a PING and wait for a PONG from the server, up to the given timeout.
	// This gives guarantee that the server has processed the above message.
	if err := ec.FlushTimeout(time.Second); err != nil {
		log.Panicln(err)
	}

	// Wait for a message to come in
	wg.Wait()
}

func DrainDemo() {

	log.Println("DrainDemo")
	nc, cancel := createNc()
	defer cancel()

	done := sync.WaitGroup{}
	done.Add(1)

	count := 0
	errCh := make(chan error, 1)

	msgAfterDrain := "not this one"

	// Just to not collide using the demo server with other users.
	subject := nats.NewInbox()

	// This callback will process each message slowly
	sub, _ := nc.Subscribe(subject, func(m *nats.Msg) {
		if string(m.Data) == msgAfterDrain {
			errCh <- errors.New("should not have received this message")
			return
		}
		time.Sleep(100 * time.Millisecond)
		count++
		if count == 2 {
			done.Done()
		}
	})

	// Send 2 messages
	for i := 0; i < 2; i++ {
		nc.Publish(subject, []byte("hello"))
	}

	// Call Drain on the subscription. It unsubscribes but
	// wait for all pending messages to be processed.
	if err := sub.Drain(); err != nil {
		log.Panicln(err)
	}

	// Send one more message, this message should not be received
	if err := nc.Publish(subject, []byte(msgAfterDrain)); err != nil {
		log.Printf("publish invalid msg:%v\n", err)
	}

	// Wait for the subscription to have processed the 2 messages.
	done.Wait()

	// Now check that the 3rd message was not received
	select {
	case e := <-errCh:
		log.Panicln(e)
	case <-time.After(200 * time.Millisecond):
		// OK!
	}
}

func QueuePubSub() {
	log.Println("QueuePubSub")
	nc, cancel := createNc()
	defer cancel()

	wg := sync.WaitGroup{}
	wg.Add(3)
	nc.QueueSubscribe("foo", "queue.foo", func(msg *nats.Msg) {
		log.Println("Subscribe 1:", string(msg.Data))
		wg.Done()
	})

	nc.QueueSubscribe("foo", "queue.foo", func(msg *nats.Msg) {
		log.Println("Subscribe 2:", string(msg.Data))
		wg.Done()
	})

	nc.QueueSubscribe("foo", "queue.foo", func(msg *nats.Msg) {
		log.Println("Subscribe 3:", string(msg.Data))
		wg.Done()
	})

	for i := 1; i <= 3; i++ {
		message := fmt.Sprintf("Message %d", i)
		if err := nc.Publish("foo", []byte(message)); err != nil {
			log.Panicln(err)
		}
	}

	wg.Wait()
}

func PubSub() {
	log.Println("PubSub")
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
			log.Panicln(err)
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
			log.Panicln(err)
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
			log.Panicln(err)
		}
		now := time.Now()
		zoneDateTime := now.In(zoneID)
		formatted := zoneDateTime.String()

		nc.Publish("time.us.east", []byte(formatted))
		nc.Publish("time.us.east.atlanta", []byte(formatted))

		zoneID, err = time.LoadLocation("Europe/Warsaw")
		if err != nil {
			log.Panicln(err)
		}
		zoneDateTime = now.In(zoneID)
		formatted = zoneDateTime.String()

		nc.Publish("time.eu.east", []byte(formatted))
		nc.Publish("time.eu.east.warsaw", []byte(formatted))
	}()
	g.Wait()
}

// Retention: nats.InterestPolicy https://natsbyexample.com/examples/jetstream/interest-stream/go
func interest_stream(nc *nats.Conn, js nats.JetStreamContext) {
	log.Println("interest_stream")
	cfg := &nats.StreamConfig{
		Name:      StreamName_EVENTS,
		Retention: nats.InterestPolicy,
		Subjects:  []string{StreamSubjects_EVENTS},
	}
	js.PurgeStream(StreamName_EVENTS)
	js.AddStream(cfg)
	defer js.DeleteStream(StreamName_EVENTS)
	log.Println("created the stream")

	js.Publish("events.page_loaded", nil)
	js.Publish("events.mouse_clicked", nil)
	ack, err := js.Publish("events.input_focused", nil)
	log.Println("published 3 messages")

	log.Printf("last message err: %v\n", err)
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

// Retention: nats.WorkQueuePolicy https://natsbyexample.com/examples/jetstream/workqueue-stream/go
func workqueue_stream(nc *nats.Conn, js nats.JetStreamContext) {
	log.Println("workqueue_stream")
	cfg := &nats.StreamConfig{
		Name:      StreamName_EVENTS,
		Retention: nats.WorkQueuePolicy,
		Subjects:  []string{StreamSubjects_EVENTS},
	}

	js.PurgeStream(StreamName_EVENTS)
	js.AddStream(cfg)
	defer js.DeleteStream(StreamName_EVENTS)
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

// Retention: nats.LimitsPolicy https://natsbyexample.com/examples/jetstream/limits-stream/go
func limits_stream(nc *nats.Conn, js nats.JetStreamContext) {
	log.Println("limits_stream")

	cfg := &nats.StreamConfig{
		Name:      StreamName_EVENTS,
		Retention: nats.LimitsPolicy,
		Subjects:  []string{StreamSubjects_EVENTS},
	}
	cfg.Storage = nats.FileStorage

	js.PurgeStream(StreamName_EVENTS)
	js.AddStream(cfg)
	defer js.DeleteStream(StreamName_EVENTS)
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
		log.Panicln("publish took too long")
	}

	printStreamState(js, cfg.Name)

	cfg.MaxMsgs = 10
	js.UpdateStream(cfg)
	log.Println("set max messages to 10")

	printStreamState(js, cfg.Name)

	cfg.MaxBytes = 300
	js.UpdateStream(cfg)
	log.Println("set max bytes to 300")

	printStreamState(js, cfg.Name)

	cfg.MaxAge = time.Second
	js.UpdateStream(cfg)
	log.Println("set max age to one second")

	printStreamState(js, cfg.Name)

	log.Println("sleeping one second...")
	time.Sleep(time.Second)

	printStreamState(js, cfg.Name)
}

// https://natsbyexample.com/examples/jetstream/pull-consumer/go
func pull_consumer(nc *nats.Conn, js nats.JetStreamContext) {
	log.Println("pull_consumer")
	cfg := &nats.StreamConfig{
		Name: StreamName_EVENTS,
		// Retention: nats.LimitsPolicy,
		Subjects: []string{StreamSubjects_EVENTS},
	}
	js.PurgeStream(StreamName_EVENTS)
	js.AddStream(cfg)
	defer js.DeleteStream(StreamName_EVENTS)
	log.Println("created the stream")

	js.Publish("events.1", nil)
	js.Publish("events.2", nil)
	js.Publish("events.3", nil)

	//临时消费者 durable=""
	sub, _ := js.PullSubscribe("", "", nats.BindStream(cfg.Name))

	ephemeralName := <-js.ConsumerNames(cfg.Name)
	log.Printf("ephemeral name is %q\n", ephemeralName)

	msgs, _ := sub.Fetch(2)
	log.Printf("got %d messages\n", len(msgs))

	msgs[0].Ack()
	msgs[1].Ack()

	msgs, _ = sub.Fetch(100)
	log.Printf("got %d messages\n", len(msgs))

	msgs[0].Ack()

	_, err := sub.Fetch(1, nats.MaxWait(time.Second))
	log.Printf("timeout? %v\n", err == nats.ErrTimeout)

	sub.Unsubscribe()

	consumerCfg := &nats.ConsumerConfig{
		Durable:   "processor",
		AckPolicy: nats.AckExplicitPolicy,
	}
	js.AddConsumer(cfg.Name, consumerCfg)

	sub1, _ := js.PullSubscribe("", consumerCfg.Durable, nats.BindStream(cfg.Name))

	msgs, _ = sub1.Fetch(1)
	log.Printf("received %q from sub1\n", msgs[0].Subject)
	msgs[0].Ack()

	sub1.Unsubscribe()
	sub1, _ = js.PullSubscribe("", consumerCfg.Durable, nats.BindStream(cfg.Name))

	msgs, _ = sub1.Fetch(1)
	log.Printf("received %q from sub1 (after reconnect)\n", msgs[0].Subject)
	msgs[0].Ack()

	sub2, _ := js.PullSubscribe("", consumerCfg.Durable, nats.BindStream(cfg.Name))

	msgs, _ = sub2.Fetch(1)
	log.Printf("received %q from sub2\n", msgs[0].Subject)
	msgs[0].Ack()

	_, err = sub1.Fetch(1, nats.MaxWait(time.Second))
	log.Printf("timeout on sub1? %v\n", err == nats.ErrTimeout)

	sub1.Unsubscribe()
	sub2.Unsubscribe()
}

// https://natsbyexample.com/examples/jetstream/multi-stream-consumption/go
func multi_stream_consumption(nc *nats.Conn, js nats.JetStreamContext) {
	log.Println("multi_stream_consumption")
	cfg1 := &nats.StreamConfig{
		Name: "EVENTS-EU",
		// Retention: nats.LimitsPolicy,
		Subjects: []string{"events.eu.>"},
	}
	js.AddStream(cfg1)
	defer js.DeleteStream(cfg1.Name)
	cfg2 := &nats.StreamConfig{
		Name: "EVENTS-US",
		// Retention: nats.LimitsPolicy,
		Subjects: []string{"events.us.>"},
	}
	js.AddStream(cfg2)
	defer js.DeleteStream(cfg2.Name)

	consumerCfg := &nats.ConsumerConfig{
		Durable:        "processor",
		DeliverSubject: "push.events",
		DeliverGroup:   "processor",
		AckPolicy:      nats.AckExplicitPolicy,
	}
	js.AddConsumer(cfg1.Name, consumerCfg)
	js.AddConsumer(cfg2.Name, consumerCfg)

	js.Publish("events.eu.page_loaded", nil)
	js.Publish("events.eu.input_focused", nil)
	js.Publish("events.us.page_loaded", nil)
	js.Publish("events.us.mouse_clicked", nil)
	js.Publish("events.eu.mouse_clicked", nil)
	js.Publish("events.us.input_focused", nil)

	sub, _ := nc.QueueSubscribeSync(consumerCfg.DeliverSubject, consumerCfg.DeliverGroup)
	defer sub.Drain()

	for {
		msg, err := sub.NextMsg(time.Second)
		if err == nats.ErrTimeout {
			break
		}

		log.Println(msg.Subject)
		msg.Ack()
	}

	info1, _ := js.ConsumerInfo(cfg1.Name, "processor")
	log.Printf("eu: last delivered: %d, num pending: %d\n", info1.Delivered.Stream, info1.NumPending)
	info2, _ := js.ConsumerInfo(cfg2.Name, "processor")
	log.Printf("us: last delivered: %d, num pending: %d\n", info2.Delivered.Stream, info2.NumPending)
}

// https://natsbyexample.com/examples/jetstream/queue-push-consumer/go
func queue_push_consumer(nc *nats.Conn, js nats.JetStreamContext) {
	log.Println("queue_push_consumer")
	cfg := &nats.StreamConfig{
		Name: StreamName_EVENTS,
		// Retention: nats.LimitsPolicy,
		Subjects: []string{StreamSubjects_EVENTS},
	}
	js.PurgeStream(StreamName_EVENTS)
	js.AddStream(cfg)
	defer js.DeleteStream(StreamName_EVENTS)
	log.Println("created the stream")
	log.Println("# Durable (implicit)")
	sub1, _ := js.QueueSubscribeSync(StreamSubjects_EVENTS, "event-processor", nats.AckExplicit())

	info, _ := js.ConsumerInfo(cfg.Name, "event-processor")
	log.Printf("deliver group: %q\n", info.Config.DeliverGroup)

	sub2, _ := js.QueueSubscribeSync(StreamSubjects_EVENTS, "event-processor", nats.AckExplicit())

	sub3, _ := nc.QueueSubscribeSync(info.Config.DeliverSubject, info.Config.DeliverGroup)
	log.Printf("deliver subject: %q\n", info.Config.DeliverSubject)

	js.Publish("events.1", nil)
	js.Publish("events.2", nil)
	js.Publish("events.3", nil)
	js.Publish("events.4", nil)
	js.Publish("events.5", nil)
	js.Publish("events.6", nil)

	msg, _ := sub1.NextMsg(time.Second)
	if msg != nil {
		log.Printf("sub1: received message %q\n", msg.Subject)
		msg.Ack()
	} else {
		log.Println("sub1: receive timeout")
	}

	msg, _ = sub2.NextMsg(time.Second)
	if msg != nil {
		log.Printf("sub2: received message %q\n", msg.Subject)
		msg.Ack()
	} else {
		log.Println("sub2: receive timeout")
	}

	msg, _ = sub3.NextMsg(time.Second)
	if msg != nil {
		log.Printf("sub3: received message %q\n", msg.Subject)
		msg.Ack()
	} else {
		log.Println("sub3: receive timeout")
	}

	sub1.Unsubscribe()
	sub2.Unsubscribe()
	sub3.Unsubscribe()

	log.Println("# Durable (explicit)")
	consumerCfg := &nats.ConsumerConfig{
		Durable:        "event-processor",
		DeliverSubject: "my-subject",
		DeliverGroup:   "event-processor",
		AckPolicy:      nats.AckExplicitPolicy,
	}
	js.AddConsumer(cfg.Name, consumerCfg)
	defer js.DeleteConsumer(cfg.Name, consumerCfg.Durable)

	wg := &sync.WaitGroup{}
	wg.Add(6)

	_, _ = nc.QueueSubscribe(consumerCfg.DeliverSubject, consumerCfg.DeliverGroup, func(msg *nats.Msg) {
		log.Printf("sub1: received message %q\n", msg.Subject)
		msg.Ack()
		wg.Done()
	})
	_, _ = nc.QueueSubscribe(consumerCfg.DeliverSubject, consumerCfg.DeliverGroup, func(msg *nats.Msg) {
		log.Printf("sub2: received message %q\n", msg.Subject)
		msg.Ack()
		wg.Done()
	})
	_, _ = nc.QueueSubscribe(consumerCfg.DeliverSubject, consumerCfg.DeliverGroup, func(msg *nats.Msg) {
		log.Printf("sub3: received message %q\n", msg.Subject)
		msg.Ack()
		wg.Done()
	})

	wg.Wait()
}

// https://natsbyexample.com/examples/jetstream/push-consumer/go
func push_consumer(nc *nats.Conn, js nats.JetStreamContext) {
	log.Println("push_consumer")
	cfg := &nats.StreamConfig{
		Name: StreamName_EVENTS,
		// Retention: nats.LimitsPolicy,
		Subjects: []string{StreamSubjects_EVENTS},
	}
	js.PurgeStream(StreamName_EVENTS)
	js.AddStream(cfg)
	defer js.DeleteStream(StreamName_EVENTS)
	log.Println("created the stream")

	js.Publish("events.1", nil)
	js.Publish("events.2", nil)
	js.Publish("events.3", nil)

	log.Println("# Ephemeral")
	sub, _ := js.SubscribeSync(StreamSubjects_EVENTS, nats.AckExplicit())

	ephemeralName := <-js.ConsumerNames(cfg.Name)
	log.Printf("ephemeral name is %q\n", ephemeralName)

	queuedMsgs, _, _ := sub.Pending()
	log.Printf("%d messages queued\n", queuedMsgs)

	js.Publish("events.4", nil)
	js.Publish("events.5", nil)
	js.Publish("events.6", nil)

	queuedMsgs, _, _ = sub.Pending()
	log.Printf("%d messages queued\n", queuedMsgs)

	msg, _ := sub.NextMsg(time.Second)
	log.Printf("received %q\n", msg.Subject)

	msg.Ack()

	msg, _ = sub.NextMsg(time.Second)
	log.Printf("received %q\n", msg.Subject)
	msg.Ack()

	queuedMsgs, _, _ = sub.Pending()
	log.Printf("%d messages queued\n", queuedMsgs)

	sub.Unsubscribe()

	log.Println("# Durable (Helper)")

	sub, _ = js.SubscribeSync(StreamSubjects_EVENTS, nats.Durable("handler-1"), nats.AckExplicit())

	queuedMsgs, _, _ = sub.Pending()
	log.Printf("%d messages queued\n", queuedMsgs)

	msg, _ = sub.NextMsg(time.Second)
	msg.Ack()

	sub.Unsubscribe()

	_, err := js.ConsumerInfo(cfg.Name, "handler-1")
	log.Println(err)

	log.Println("# Durable (AddConsumer)")
	consumerCfg := &nats.ConsumerConfig{
		Durable:        "handler-2",
		DeliverSubject: "handler-2",
		AckPolicy:      nats.AckExplicitPolicy,
		AckWait:        time.Second,
	}
	js.AddConsumer(cfg.Name, consumerCfg)

	sub, _ = js.SubscribeSync("", nats.Bind(cfg.Name, consumerCfg.Durable))

	msg, _ = sub.NextMsg(time.Second)
	log.Printf("received %q\n", msg.Subject)

	msg.Ack()
	queuedMsgs, _, _ = sub.Pending()
	log.Printf("%d messages queued\n", queuedMsgs)

	sub.Unsubscribe()

	info, _ := sub.ConsumerInfo()
	log.Printf("max stream sequence delivered: %d\n", info.Delivered.Stream)
	log.Printf("max consumer sequence delivered: %d\n", info.Delivered.Consumer)
	log.Printf("num ack pending: %d\n", info.NumAckPending)
	log.Printf("num redelivered: %d\n", info.NumRedelivered)

	sub, _ = js.SubscribeSync("", nats.Bind(cfg.Name, consumerCfg.Durable))
	_, err = sub.NextMsg(100 * time.Millisecond)
	log.Printf("received timeout? %v\n", err == nats.ErrTimeout)

	msg, _ = sub.NextMsg(time.Second)
	md, _ := msg.Metadata()
	log.Printf("received %q (delivery #%d)\n", msg.Subject, md.NumDelivered)
	msg.Ack()

	info, _ = sub.ConsumerInfo()
	log.Printf("max stream sequence delivered: %d\n", info.Delivered.Stream)
	log.Printf("max consumer sequence delivered: %d\n", info.Delivered.Consumer)
	log.Printf("num ack pending: %d\n", info.NumAckPending)
	log.Printf("num redelivered: %d\n", info.NumRedelivered)
}

// https://natsbyexample.com/examples/jetstream/pull-consumer-limits/go
func pull_consumer_limits(nc *nats.Conn, js nats.JetStreamContext) {
	log.Println("pull_consumer_limits")
	cfg := &nats.StreamConfig{
		Name: StreamName_EVENTS,
		// Retention: nats.LimitsPolicy,
		Subjects: []string{StreamSubjects_EVENTS},
	}
	js.PurgeStream(StreamName_EVENTS)
	js.AddStream(cfg)
	defer js.DeleteStream(StreamName_EVENTS)
	log.Println("created the stream")

	consumerName := "processor"
	ackWait := 10 * time.Second
	maxWaiting := 1
	ackPolicy := nats.AckExplicitPolicy
	js.AddConsumer(cfg.Name, &nats.ConsumerConfig{
		Durable:    consumerName,
		AckPolicy:  ackPolicy,
		AckWait:    ackWait,
		MaxWaiting: maxWaiting,
	})

	sub, _ := js.PullSubscribe("", consumerName, nats.Bind(cfg.Name, consumerName))

	log.Println("--- max in-flight messages (n=1) ---")
	_, err := js.UpdateConsumer(cfg.Name, &nats.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     ackPolicy,
		AckWait:       ackWait,
		MaxWaiting:    maxWaiting,
		MaxAckPending: 1,
	})
	log.Printf("%v\n", err)

	js.Publish("events.1", nil)
	js.Publish("events.2", nil)

	msgs, _ := sub.Fetch(3)
	log.Printf("requested 3, got %d\n", len(msgs))
	for i, v := range msgs {
		log.Printf("msg[%d]=%v\n", i, v.Subject)
	}

	_, err = sub.Fetch(1, nats.MaxWait(time.Second))
	log.Printf("%v\n", err)

	msgs[0].Ack()

	msgs, _ = sub.Fetch(1)
	log.Printf("requested 1, got %d\n", len(msgs))
	for i, v := range msgs {
		log.Printf("msg[%d]=%v\n", i, v.Subject)
	}
	msgs[0].Ack()

	log.Println("--- max fetch batch size (n=2) ---")
	js.UpdateConsumer(cfg.Name, &nats.ConsumerConfig{
		Durable:         consumerName,
		AckPolicy:       ackPolicy,
		AckWait:         ackWait,
		MaxWaiting:      maxWaiting,
		MaxRequestBatch: 2,
	})

	js.Publish("events.1", nil)
	js.Publish("events.2", nil)

	_, err = sub.Fetch(10)
	log.Printf("%v\n", err)

	msgs, _ = sub.Fetch(2)
	log.Printf("requested 2, got %d\n", len(msgs))
	for i, v := range msgs {
		log.Printf("msg[%d]=%v\n", i, v.Subject)
	}

	msgs[0].Ack()
	msgs[1].Ack()

	log.Println("--- max waiting requests (n=1) ---")

	js.UpdateConsumer(cfg.Name, &nats.ConsumerConfig{
		Durable:    consumerName,
		AckPolicy:  ackPolicy,
		AckWait:    ackWait,
		MaxWaiting: maxWaiting,
	})

	log.Printf("is valid? %v\n", sub.IsValid())

	wg := &sync.WaitGroup{}
	wg.Add(3)

	go func() {
		_, err := sub.Fetch(1, nats.MaxWait(time.Second))
		log.Printf("fetch 1: %s\n", err)
		wg.Done()
	}()

	go func() {
		_, err := sub.Fetch(1, nats.MaxWait(time.Second))
		log.Printf("fetch 2: %s\n", err)
		wg.Done()
	}()

	go func() {
		_, err := sub.Fetch(1, nats.MaxWait(time.Second))
		log.Printf("fetch 3: %s\n", err)
		wg.Done()
	}()

	wg.Wait()

	log.Println("--- max fetch timeout (d=1s) ---")

	js.UpdateConsumer(cfg.Name, &nats.ConsumerConfig{
		Durable:           consumerName,
		AckPolicy:         ackPolicy,
		AckWait:           ackWait,
		MaxRequestExpires: time.Second,
	})

	t0 := time.Now()
	_, err = sub.Fetch(1, nats.MaxWait(time.Second))
	log.Printf("timeout occured? %v in %s\n", err == nats.ErrTimeout, time.Since(t0))

	t0 = time.Now()
	_, err = sub.Fetch(1, nats.MaxWait(5*time.Second))
	log.Printf("%s in %s\n", err, time.Since(t0))

}
