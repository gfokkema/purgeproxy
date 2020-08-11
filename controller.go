package main

import (
	"fmt"
	"time"

	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	klog "k8s.io/klog/v2"

	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

// EndpointLoggingController logs the name and namespace of endpoints that are added, deleted, or updated
type EndpointLoggingController struct {
	informerFactory  informers.SharedInformerFactory
	endpointInformer coreinformers.EndpointsInformer
}

// Run starts shared informers and waits for the shared informer cache to synchronize.
func (c *EndpointLoggingController) Run(stopCh chan struct{}) error {
	c.informerFactory.Start(stopCh)
	if !cache.WaitForCacheSync(stopCh, c.endpointInformer.Informer().HasSynced) {
		return fmt.Errorf("Failed to sync")
	}
	return nil
}

func (c *EndpointLoggingController) endpointAdd(obj interface{}) {
	endpoint := obj.(*v1.Endpoints)
	klog.Infof("ENDPOINT CREATED: %s/%s", endpoint.Namespace, endpoint.Name)
}

func (c *EndpointLoggingController) endpointUpdate(old, new interface{}) {
	oldEndpoint := old.(*v1.Endpoints)
	newEndpoint := new.(*v1.Endpoints)
	klog.Infof("OLD: %#v\n", oldEndpoint)
	klog.Infof("NEW: %#v\n", newEndpoint)
	klog.Infof(
		"ENDPOINT UPDATED. %s/%s",
		oldEndpoint.Namespace, oldEndpoint.Name,
	)
	for _, sub := range newEndpoint.Subsets {
		for _, addr := range sub.Addresses {
			klog.Infof(
				"(%s, %s)\n", addr.IP, addr.TargetRef.Name,
			)
		}
	}
}

func (c *EndpointLoggingController) endpointDelete(obj interface{}) {
	endpoint := obj.(*v1.Endpoints)
	klog.Infof("ENDPOINT DELETED: %s/%s", endpoint.Namespace, endpoint.Name)
}

// NewEndpointLoggingController creates a EndpointLoggingController
func NewEndpointLoggingController(clientset *kubernetes.Clientset) *EndpointLoggingController {
	informerFactory := informers.NewFilteredSharedInformerFactory(clientset, time.Hour, "htm", func(o *metaV1.ListOptions) {})
	endpointInformer := informerFactory.Core().V1().Endpoints()
	c := &EndpointLoggingController{
		informerFactory:  informerFactory,
		endpointInformer: endpointInformer,
	}
	endpointInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.endpointAdd,
			UpdateFunc: c.endpointUpdate,
			DeleteFunc: c.endpointDelete,
		},
	)
	return c
}
