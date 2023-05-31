package main

import (
	"context"
	"encoding/binary"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

var (
	// nats://...
	// ws://...
	// tls://...
	servers = []string{
		"nats://127.0.0.1:4222,nats://127.0.0.1:4223,nats://127.0.0.1:4224",
	}
	User = "app"
	//cmd:nats server passwd
	Pass = "app"
)

const (
	StreamName_EVENTS     = "EVENTS"
	StreamSubjects_EVENTS = "events.>"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Llongfile)
}

// Connect to NATS
func createNc() (nc *nats.Conn, deferFunc func()) {
	var err error
	nc, err = nats.Connect(strings.Join(servers, ","),
		nats.UserInfo(User, Pass),
		nats.Timeout(10*time.Second),
		nats.PingInterval(10*time.Second),
		nats.MaxPingsOutstanding(3),
		// nats.MaxReconnects(5),
		// nats.NoReconnect(),
		// nats.DontRandomize(),
		// nats.ReconnectWait(10*time.Second),
		// nats.ReconnectBufSize(512),
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
func main() {
	/*nc*/ _, js, cancel := createJs()
	// log.Printf("nc:%+v,js:%+v\n", nc, js)
	defer cancel()
	wg := &sync.WaitGroup{}
	ctx, cancelFunc := context.WithCancel(context.Background())
	closeCh := make(chan os.Signal)
	signal.Notify(closeCh, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case signalV := <-closeCh:
				log.Printf("sys signal:%v\n", signalV)
				cancelFunc()
				return
			}
		}
	}()
	streamCfg := &nats.StreamConfig{
		Name:      StreamName_EVENTS,
		Retention: nats.WorkQueuePolicy,
		Subjects:  []string{StreamSubjects_EVENTS},
		Discard:   nats.DiscardOld,
		Storage:   nats.FileStorage,
		Replicas:  3,
	}
	/*sInfo, err :=*/ js.AddStream(streamCfg)
	// log.Printf("streamInfo:%+v,err:%v\n", sInfo, err)
	consumerCfg := &nats.ConsumerConfig{
		Durable:       "processor",
		FilterSubject: "events.*",
		AckPolicy:     nats.AckExplicitPolicy,
	}
	/*cInfo, err := */ js.AddConsumer(streamCfg.Name, consumerCfg)
	// log.Printf("consumerInfo:%+v,err:%v\n", cInfo, err)

	var consumerFunc = func(ctx context.Context, wg *sync.WaitGroup, streamName string, consumerCfg *nats.ConsumerConfig) {
		defer wg.Done()
		defer func() {
			if err := recover(); err != nil {
				log.Printf("consumer:%+v,Error:%v\n", consumerCfg, err)
			}
		}()
		sub, err := js.PullSubscribe(consumerCfg.FilterSubject, consumerCfg.Durable, nats.Bind(streamName, consumerCfg.Durable))
		if err != nil {
			panic(err)
		}
		// var num uint32
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			msgs, err := sub.Fetch(1, nats.MaxWait(1*time.Second))
			if err != nil {
				// log.Printf("fetch err:%v\n", err)
			}
			for _, tMsg := range msgs {
				// num = binary.BigEndian.Uint32(tMsg.Data)
				// log.Printf("msg:%+v\n", num)
				tMsg.AckSync()
			}
		}
	}
	wg.Add(1)
	go consumerFunc(ctx, wg, streamCfg.Name, consumerCfg)

	var productFunc = func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		var num uint32
		bf := make([]byte, 4)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			num++
			binary.BigEndian.PutUint32(bf[0:4], num)
			ack, err := js.Publish("events.num", bf)
			if err != nil {
				log.Printf("publish %v ack:%v err:%v\n", num, ack, err)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	wg.Add(1)
	go productFunc(ctx, wg)
	wg.Wait()
}
