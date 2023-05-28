package main

import (
	"context"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
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

type Instance struct {
	Name           string
	RequestCount   int
	InstanceStatus InstanceStatus
	WorkRate       int
}

type Instances struct {
	Instances *map[string]Instance
}

type HandlerData struct {
	data   *map[string]Instance
	killed *map[string]time.Time
	mutex  *sync.Mutex
}

type PubsubParameters struct {
	TopicName        string
	SubscriptionName string
	ProjectId        string
}

func main() {
	var topic, instanceName, projectId string
	if topic = os.Getenv("TOPIC_NAME"); topic == "" {
		log.Fatalln("No topic name given for CloudRun.")
	}

	if projectId = os.Getenv("PROJECT_ID"); projectId == "" {
		log.Fatalln("No project id given.")
	}
	instanceName = randName()

	params := &PubsubParameters{
		TopicName:        topic,
		SubscriptionName: "subscription" + instanceName,
		ProjectId:        projectId,
	}

	instanceData := make(map[string]Instance)
	killedInstances := make(map[string]time.Time)

	handler := &HandlerData{data: &instanceData, killed: &killedInstances, mutex: &sync.Mutex{}}
	go handler.setupPubsub(params)
	renderPage(&instanceData)
	go removeDeletedInstances(handler)
	servePages()
}

func handleSigterm(sub *pubsub.Subscription, ctx *context.Context) {
	terminateSignal := make(chan os.Signal, 1)
	signal.Notify(terminateSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGABRT)
	for {
		_, ok := <-terminateSignal
		if ok {
			if err := sub.Delete(*ctx); err != nil {
				log.Panicln("Delete: ", err)
			}
			log.Println(" Exiting...")
			os.Exit(0)
		}
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

func renderPage(instanceData *map[string]Instance) {
	tmpl := template.Must(template.ParseFiles("instances.html"))
	pagePath := "/"
	http.HandleFunc(pagePath, func(w http.ResponseWriter, r *http.Request) {
		instances := Instances{
			Instances: instanceData,
		}
		if err := tmpl.Execute(w, instances); err != nil {
			log.Fatalln("Failed template execution ", err)
		}
	})
}

func servePages() {
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalln("Receive execute listen and serve failed ", err)
	}
}

func (h *HandlerData) setupPubsub(params *PubsubParameters) {
	ctx := context.Background()
	var client *pubsub.Client
	var err error
	if client, err = pubsub.NewClient(ctx, params.ProjectId); err != nil {
		log.Fatalf("Could not create pubsub Client: %v", err)
	}

	topic := client.Topic(params.TopicName)
	found, _ := topic.Exists(ctx)
	if !found {
		log.Fatalf("topic not found: %s", params.TopicName)
	}

	subscription, err := client.CreateSubscription(context.Background(),
		params.SubscriptionName,
		pubsub.SubscriptionConfig{Topic: topic})
	if err != nil {
		subscription = client.Subscription(params.SubscriptionName)
	}

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go handleSigterm(subscription, &subCtx)
	err = subscription.Receive(subCtx, h.handleMessage)
	if err != nil {
		log.Fatalf("Receive function failed with %v", err)
	}
}

func removeDeletedInstances(h *HandlerData) {
	for {
		timer := time.NewTicker(time.Duration(5) * time.Second)
		_, ok := <-timer.C
		if ok {
			h.mutex.Lock()
			for k, v := range *(*h).killed {
				diff := time.Since(v)
				if diff.Seconds() > 60 {
					delete((*(*h).data), k)
				}
			}
			h.mutex.Unlock()
		}
	}
}

func (h *HandlerData) handleMessage(ctx context.Context, msg *pubsub.Message) {
	msg.Ack()
	var instanceMessage Instance
	json.Unmarshal(msg.Data, &instanceMessage)

	h.mutex.Lock()
	(*(*h).data)[instanceMessage.Name] = instanceMessage
	if instanceMessage.InstanceStatus == Killed {
		(*(*h).killed)[instanceMessage.Name] = time.Now()
	}
	h.mutex.Unlock()
}
