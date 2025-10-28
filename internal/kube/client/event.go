package client

import (
	"log"
	"log/slog"
	"sync"
	"time"

	"skupper-cert-manager/internal/logger"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const maxRequeues = 5

type EventInformer interface {
	Informer() cache.SharedIndexInformer
	Handle(key string) error
}
type Event struct {
	Key     string
	Handler EventInformer
}

func NewEventProcessor(namespace string) *EventProcessor {
	return &EventProcessor{
		queue:  workqueue.NewTypedRateLimitingQueue[Event](workqueue.DefaultTypedControllerRateLimiter[Event]()),
		logger: logger.NewLogger("event-processor", namespace),
	}
}

type EventProcessor struct {
	eventInformers []EventInformer
	queue          workqueue.TypedRateLimitingInterface[Event]
	started        bool
	mutex          sync.Mutex
	logger         *slog.Logger
}

func (e *EventProcessor) AddInformer(ei EventInformer) error {
	_, err := ei.Informer().AddEventHandler(e.newEventHandler(ei))
	if err != nil {
		return err
	}
	e.eventInformers = append(e.eventInformers, ei)
	return nil
}

func (e *EventProcessor) newEventHandler(handler EventInformer) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := KeyFromObj(obj)
			if err != nil {
				log.Println("Error parsing obj key:", err)
				return
			}
			event := Event{
				Key:     key,
				Handler: handler,
			}
			e.queue.Add(event)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := KeyFromObj(newObj)
			if err != nil {
				log.Println("Error parsing obj key:", err)
				return
			}
			event := Event{
				Key:     key,
				Handler: handler,
			}
			e.queue.Add(event)
		},
		DeleteFunc: func(obj interface{}) {
			key, err := KeyFromObj(obj)
			if err != nil {
				log.Println("Error parsing obj key:", err)
				return
			}
			event := Event{
				Key:     key,
				Handler: handler,
			}
			e.queue.Add(event)
		},
	}
}

func (e *EventProcessor) StartInformers(stopCh <-chan struct{}) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if e.started {
		e.logger.Debug("Already started")
		return
	}
	for _, eventInformer := range e.eventInformers {
		go eventInformer.Informer().Run(stopCh)
	}
	e.started = true
}

func (e *EventProcessor) Start(stopCh <-chan struct{}) {
	go wait.Until(e.run, time.Second, stopCh)
}

func (e *EventProcessor) run() {
	for e.process() {
	}
}

func (e *EventProcessor) process() bool {
	event, shutdown := e.queue.Get()
	if shutdown {
		return false
	}
	defer e.queue.Done(event)
	err := event.Handler.Handle(event.Key)
	if err != nil {
		requeues := e.queue.NumRequeues(event)
		if requeues > maxRequeues {
			e.queue.Forget(event)
			e.logger.Debug("unable to re-queue after processing time", "key", event.Key)
			return true
		}
		e.queue.AddRateLimited(event)
		return true
	}
	e.queue.Forget(event)
	return true
}

func KeyFromObj(obj interface{}) (string, error) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return "", err
	}
	return key, nil
}
