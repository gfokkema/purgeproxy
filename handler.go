package main

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"
)

type purgeHandler struct {
	client      *http.Client
	addressList *AddressList
}

func (p *purgeHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	results := p.addressList.Execute(func(addr string, endpoint v1.EndpointAddress) (interface{}, error) {
		url, err := url.Parse(fmt.Sprintf("http://%s%s", addr, req.URL.Path))
		if err != nil {
			return nil, err
		}

		klog.Infof("[%s] Requesting url %s\n", addr, url)
		proxyreq := &http.Request{
			Method: "BAN",
			URL:    url,
			Header: req.Header,
			Body:   req.Body,
		}
		return p.client.Do(proxyreq)
	})

	code := http.StatusOK
	body := ""
	for result := range results {
		if result.err != nil {
			code = http.StatusInternalServerError
			klog.Errorf("[%s] Error occurred: %s", result.addr, result.err)
			body += fmt.Sprintf("%s\n", result.err)
			continue
		}

		klog.Infof("[%s] Result: %s", result.addr, result.result)
		body += fmt.Sprintf("[%s] %s", result.addr, result.result)
	}
	w.WriteHeader(code)
	w.Write([]byte(body))
}

func NewPurgeHandler(addressList *AddressList) *purgeHandler {
	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}
	h := &purgeHandler{
		&http.Client{Transport: tr, Timeout: 5 * time.Second},
		addressList,
	}

	return h
}
