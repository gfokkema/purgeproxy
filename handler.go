package main

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"k8s.io/klog"
)

type purgeHandler struct {
	client      *http.Client
	addressList *AddressList
}

func (p *purgeHandler) GetExecuteFunc(req *http.Request) ExecuteFunc {
	return func(addr Address, endpoint AddressRef) (interface{}, error) {
		url, err := url.Parse(fmt.Sprintf("http://%s%s", addr, req.URL.Path))
		if err != nil {
			return nil, err
		}

		klog.Infof("[%s] Requesting url %s\n", addr, url)
		proxyreq := &http.Request{
			Method: req.Method,
			URL:    url,
			Header: req.Header,
			Body:   req.Body,
		}
		return p.client.Do(proxyreq)
	}
}

func (p *purgeHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	results := p.addressList.Execute(p.GetExecuteFunc(req))
	code := http.StatusOK
	body := ""
	for result := range results {
		if result.err != nil {
			code = http.StatusInternalServerError
			klog.Errorf("[%s] Error: %s", result.addr, result.err)
			body += fmt.Sprintf("[%v] Error: %s\n", result.addr, result.err)
			continue
		}

		klog.Infof("[%s] OK: %s", result.addr, result.result)
		body += fmt.Sprintf("[%v] OK: %s", result.addr, result.result)
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
