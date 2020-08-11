package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

func inConfig() (*rest.Config, error) {
	return rest.InClusterConfig()
}

func outConfig() (*rest.Config, error) {
	var kubeconfig *string
	if home := os.Getenv("HOME"); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	return clientcmd.BuildConfigFromFlags("", *kubeconfig)
}

type purgeHandler struct {
}

func (p *purgeHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	fmt.Println("PurgeHandler")
}

func main() {
	// config, err := inConfig()
	config, err := outConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	controller := NewEndpointLoggingController(clientset)
	stop := make(chan struct{})
	defer close(stop)

	go func() {
		err = controller.Run(stop)
		if err != nil {
			klog.Fatal(err)
		}
	}()

	r := mux.NewRouter()
	r.Methods("PURGE", "BAN").Handler(&purgeHandler{})
	s := &http.Server{
		Addr:           ":8080",
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())
}
