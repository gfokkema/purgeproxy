package main

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

type Address struct {
	ip   string
	port int32
}

func (a Address) String() string {
	return fmt.Sprintf("%s:%d", a.ip, a.port)
}

type AddressRef struct {
	ip   v1.EndpointAddress
	port v1.EndpointPort
}

func (a AddressRef) String() string {
	return fmt.Sprintf("(%s:%d, %s)", a.ip.IP, a.port.Port, a.ip.TargetRef.Name)
}

// AddressList contains a unique list of IPs
type AddressList struct {
	list map[Address]AddressRef
	mux  sync.RWMutex
}

// Add the values contained in `endpoint` to AddressList
func (l *AddressList) Add(endpoint *v1.Endpoints) {
	l.mux.Lock()
	defer l.mux.Unlock()

	for _, sub := range endpoint.Subsets {
		// From the docs on EndpointSubset:
		// "The expanded set of endpoints is the Cartesian product of Addresses x Ports."
		for _, addr := range sub.Addresses {
			for _, port := range sub.Ports {
				l.list[Address{addr.IP, port.Port}] = AddressRef{addr, port}
			}
		}
	}
}

// Remove the values contained in `endpoint` from AddressList
func (l *AddressList) Remove(endpoint *v1.Endpoints) {
	l.mux.Lock()
	defer l.mux.Unlock()

	for _, sub := range endpoint.Subsets {
		// From the docs on EndpointSubset:
		// "The expanded set of endpoints is the Cartesian product of Addresses x Ports."
		for _, addr := range sub.Addresses {
			for _, port := range sub.Ports {
				delete(l.list, Address{addr.IP, port.Port})
			}
		}
	}
}

// ExecuteFunc will be executed on each entry in AddressList
type ExecuteFunc func(Address, AddressRef) (interface{}, error)

// ExecuteResult stores the result of executing ExecuteFunc
type ExecuteResult struct {
	addr   Address
	result interface{}
	err    error
}

// Execute f on all values contained in AddressList
func (l *AddressList) Execute(f ExecuteFunc) chan ExecuteResult {
	var wg sync.WaitGroup
	c := make(chan ExecuteResult)

	func() {
		l.mux.RLock()
		defer l.mux.RUnlock()

		for addr, endpoint := range l.list {
			wg.Add(1)
			go func(addr Address, endpoint AddressRef, c chan ExecuteResult) {
				defer wg.Done()
				resp, err := f(addr, endpoint)
				c <- ExecuteResult{addr, resp, err}
			}(addr, endpoint, c)
		}
	}()

	go func() {
		wg.Wait()
		close(c)
	}()
	return c
}

// List all values contained in AddressList
func (l *AddressList) List() {
	l.mux.RLock()
	l.mux.RUnlock()

	klog.Infoln("LIST:")
	for _, endpoint := range l.list {
		klog.Info(endpoint)
	}
}

// NewAddressList returns an empty list of addressList
func NewAddressList() *AddressList {
	return &AddressList{
		make(map[Address]AddressRef),
		sync.RWMutex{},
	}
}

// EndpointLoggingController logs the name and namespace of endpoints that are added, deleted, or updated
type EndpointLoggingController struct {
	informerFactory  informers.SharedInformerFactory
	endpointInformer coreinformers.EndpointsInformer
	addressList      *AddressList
	mux              sync.Mutex
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
	c.addressList.Add(endpoint)
	c.addressList.List()
}

func (c *EndpointLoggingController) endpointUpdate(old, new interface{}) {
	oldEndpoint := old.(*v1.Endpoints)
	newEndpoint := new.(*v1.Endpoints)
	klog.Infof("ENDPOINT UPDATED: %s/%s\n", oldEndpoint.Namespace, oldEndpoint.Name)
	c.addressList.Remove(oldEndpoint)
	c.addressList.Add(newEndpoint)
	c.addressList.List()
}

func (c *EndpointLoggingController) endpointDelete(obj interface{}) {
	endpoint := obj.(*v1.Endpoints)
	klog.Infof("ENDPOINT DELETED: %s/%s", endpoint.Namespace, endpoint.Name)
	c.addressList.Add(&v1.Endpoints{})
	c.addressList.List()
}

// NewEndpointLoggingController creates a EndpointLoggingController
func NewEndpointLoggingController(clientset *kubernetes.Clientset, addressList *AddressList, namespace string, selector string) *EndpointLoggingController {
	informerFactory := informers.NewSharedInformerFactoryWithOptions(
		clientset, time.Hour,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(
			func(l *metav1.ListOptions) {
				l.LabelSelector = selector
			},
		))
	endpointInformer := informerFactory.Core().V1().Endpoints()

	c := &EndpointLoggingController{
		informerFactory:  informerFactory,
		endpointInformer: endpointInformer,
		addressList:      addressList,
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
