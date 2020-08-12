package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

func main() {
	pflag.Bool("kubecluster", false, "(optional) specifies whether run in cluster or not")
	pflag.String("kubeconfig", "", "(optional) absolute path to the kubeconfig file")
	pflag.String("kubenamespace", "", "Specifies the namespace in which to run")
	pflag.String("kubeselector", "", "Specifies the label selector to use for targeted endpoints")
	pflag.Parse()

	if home := os.Getenv("HOME"); home != "" {
		viper.SetDefault("kubeconfig", filepath.Join(home, ".kube", "config"))
	}
	viper.SetDefault("kubeselector", "app.kubernetes.io/type=varnish")
	viper.AutomaticEnv()
	viper.BindPFlags(pflag.CommandLine)

	var (
		config *rest.Config
		err    error
	)
	if viper.GetBool("kubecluster") {
		// use the in cluster config
		config, err = rest.InClusterConfig()
	} else {
		// use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", viper.GetString(("kubeconfig")))
	}
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	addressList := NewAddressList()
	controller := NewEndpointLoggingController(clientset, addressList, viper.GetString("kubenamespace"), viper.GetString("kubeselector"))
	stop := make(chan struct{})
	defer close(stop)

	go func() {
		err = controller.Run(stop)
		if err != nil {
			klog.Fatal(err)
		}
	}()

	h := NewPurgeHandler(addressList)
	r := mux.NewRouter()
	r.Methods("PURGE", "BAN").Handler(h)
	s := &http.Server{
		Addr:           ":8080",
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())
}
