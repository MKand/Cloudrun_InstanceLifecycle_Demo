package main

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/lucasepe/codename"
)

type InstanceStatus int

const (
	Unknown InstanceStatus = iota
	Idle
	Processing
	Killed
)

type TopicPublisher struct {
	Topic *pubsub.Topic
}

type InstanceStatusMessage struct {
	Name           string
	RequestCount   int
	InstanceStatus InstanceStatus
	WorkRate       int
}

type HelloData struct {
	RequestCount   int
	ActiveRequests int
	Name           string
	Deleted        bool
	WorkRate       int
}

type Channels struct {
	StartRequest    chan bool
	FinishedRequest chan bool
	WorkFinished    chan int
}

func main() {
	var topicName, projectId string
	if projectId = os.Getenv("PROJECT_ID"); projectId == "" {
		log.Fatalln("No project id given.")
	}
	if topicName = os.Getenv("TOPIC_NAME"); topicName == "" {
		log.Fatalln("No topic name given.")
	}

	topic := setupPubsub(projectId, topicName)
	tp := &TopicPublisher{
		Topic: topic,
	}
	h := &HelloData{
		RequestCount:   0,
		ActiveRequests: 0,
		Name:           randName(),
		Deleted:        false,
		WorkRate:       0,
	}

	fmt.Println("Starting instance: ", h.Name, "...")
	tp.writeMessage(h)
	channels := initChannels(h, tp)
	go doWork(channels.WorkFinished)
	handleRequests(h.Name, channels.StartRequest, channels.FinishedRequest)
}

func initChannels(h *HelloData, tp *TopicPublisher) Channels {
	terminateSignal := make(chan os.Signal, 1)
	channels := Channels{
		StartRequest:    make(chan bool),
		FinishedRequest: make(chan bool),
		WorkFinished:    make(chan int),
	}
	signal.Notify(terminateSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGABRT)
	go handleChannels(h, tp, channels, terminateSignal)
	return channels
}

func handleChannels(h *HelloData, tp *TopicPublisher, channels Channels, terminate <-chan os.Signal) {
	intervalString := os.Getenv("MESSAGE_INTERVAL")
	var interval int
	var err error
	if interval, err = strconv.Atoi(intervalString); err != nil {
		interval = 1
	}
	messageTimer := time.NewTicker(time.Duration(interval) * time.Second)
	for {
		select {
		case <-messageTimer.C:
			tp.writeMessage(h)

		case <-channels.StartRequest:
			h.ActiveRequests++
			tp.writeMessage(h)

		case <-channels.FinishedRequest:
			h.RequestCount++
			h.ActiveRequests--

		case rate := <-channels.WorkFinished:
			h.WorkRate = rate

		case <-terminate:
			h.Deleted = true
			tp.writeMessage(h)
			fmt.Println(" Exiting...", h)
			os.Exit(0)
		}
	}
}

func handleRequests(name string, startRequest chan bool, finishedRequest chan bool) {
	http.HandleFunc("/", handleHello(name, startRequest, finishedRequest))
	http.HandleFunc("/health", healthCheck())
	http.ListenAndServe(":8080", nil)
}

func handleHello(name string, startRequest chan bool, finishedRequest chan bool) http.HandlerFunc {
	delay := 10
	delay, _ = strconv.Atoi(os.Getenv("RESPONSE_DELAY_INTERVAL"))

	return func(w http.ResponseWriter, r *http.Request) {
		startRequest <- true
		timer := time.NewTimer(time.Duration(delay) * time.Second)
		<-timer.C
		message := fmt.Sprintf("Hi, from instance: %s \n", name)
		fmt.Println(message)
		w.Write([]byte(message))
		finishedRequest <- true
	}
}

func doWork(rate chan int) {
	for {
		timer := time.NewTimer(2 * time.Second)
		hashCount := 0
		counting := true
		for {
			if counting {
				select {
				case <-timer.C:
					counting = false
					//Taking average per ms
					rate <- int(hashCount / 2000)
				default:
					h := sha512.New()
					h.Write([]byte("This is a random string"))
					_ = hex.EncodeToString(h.Sum(nil))
					hashCount++
				}
			} else {
				break
			}
		}
	}
}

func setupPubsub(projectId string, topicName string) *pubsub.Topic {
	var client *pubsub.Client
	var err error
	ctx := context.Background()
	if client, err = pubsub.NewClient(ctx, projectId); err != nil {
		log.Fatalf("Could not create pubsub Client: %v", err)
	}
	return client.Topic(topicName)
}

func (tp *TopicPublisher) publish(msg string) error {
	ctx := context.Background()
	result := tp.Topic.Publish(ctx, &pubsub.Message{
		Data: []byte(msg),
	})
	_, err := result.Get(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (tp *TopicPublisher) writeMessage(data *HelloData) {

	message := &InstanceStatusMessage{
		RequestCount: data.RequestCount,
		Name:         data.Name,
		WorkRate:     data.WorkRate,
	}
	if data.Deleted {
		message.InstanceStatus = Killed
	} else if data.ActiveRequests > 0 {
		message.InstanceStatus = Processing
	} else {
		message.InstanceStatus = Idle
	}

	b, _ := json.Marshal(message)
	log.Println("Publish: ", string(b))
	if err := tp.publish(string(b)); err != nil {
		log.Fatalf("Failed to publish: %v", err)
	}
}

func randName() string {
	rng, err := codename.DefaultRNG()
	if err != nil {
		panic(err)
	}
	name := codename.Generate(rng, 0)
	return name
}

func healthCheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
	}
}
