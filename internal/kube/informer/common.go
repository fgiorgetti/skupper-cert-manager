package informer

import (
	"fmt"
	"time"

	"skupper-cert-manager/internal/kube/client"
)

const (
	resyncPeriod = 30 * time.Second
)

type ActionHandler[T any] interface {
	Filter(obj T) bool
	Add(key string, obj T) error
	Delete(key string) error
	Update(key string, old, new T) error
	Reconcile(key string, new T) error
	Cache() map[string]T
	Equal(oldObj, newObj T) bool
	client.EventInformer
}

func Handle[T any](key string, handler ActionHandler[T]) error {
	obj, exists, err := handler.Informer().GetStore().GetByKey(key)
	if err != nil {
		return fmt.Errorf("error retrieving key from informer store: %v", err)
	}
	oldObj, ok := handler.Cache()[key]
	if !exists {
		if ok {
			return handler.Delete(key)
		}
		// removed unhandled key
		return nil
	}
	newObj := obj.(T)
	if !handler.Filter(newObj) {
		return nil
	}
	if !ok {
		return handler.Add(key, newObj)
	}
	if !handler.Equal(oldObj, newObj) {
		return handler.Update(key, oldObj, newObj)
	}
	return handler.Reconcile(key, newObj)
}
