package router

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"

	log "github.com/sirupsen/logrus"
)

type ProducerProxyHandler interface {
	Director(req *http.Request)
	ErrorHandler(rw http.ResponseWriter, req *http.Request, err error)
}

type producerProxyHandler struct {
	target    *url.URL
	profile   Profile
	appSecret string
}

func NewProducerProxyHandler(target *url.URL, profile Profile, appSecret string) ProducerProxyHandler {
	return &producerProxyHandler{
		target:    target,
		profile:   profile,
		appSecret: appSecret,
	}
}

func (h producerProxyHandler) Director(req *http.Request) {
	target := h.target
	targetQuery := target.RawQuery

	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	req.URL.Path = singleJoiningSlash(target.Path, req.URL.Path)
	if targetQuery == "" || req.URL.RawQuery == "" {
		req.URL.RawQuery = targetQuery + req.URL.RawQuery
	} else {
		req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
	}
	if _, ok := req.Header["User-Agent"]; !ok {
		// explicitly disable User-Agent so it's not set to default value
		req.Header.Set("User-Agent", "")
	}

	// generate new body
	var timber map[string]interface{}
	b, _ := ioutil.ReadAll(req.Body)

	err := json.Unmarshal(b, &timber)

	if err != nil {
		log.Errorf("%s", err.Error())
		return
	}

	timber["_ctx"] = h.timberContext()

	b, _ = json.Marshal(timber)

	req.ContentLength = int64(len(b))
	req.Body = ioutil.NopCloser(bytes.NewReader(b))

}

func (h producerProxyHandler) timberContext() TimberContext {
	return TimberContext{
		KafkaTopic:             h.profile.Meta.Kafka.TopicName,
		KafkaPartition:         h.profile.Meta.Kafka.Partition,
		KafkaReplicationFactor: h.profile.Meta.Kafka.ReplicationFactor,
		ESIndexPrefix:          h.profile.Meta.Elasticsearch.IndexPrefix,
		ESDocumentType:         h.profile.Meta.Elasticsearch.DocumentType,
		AppMaxTPS:              h.profile.MaxTps,
		AppSecret:              h.appSecret,
	}
}

func (h producerProxyHandler) ErrorHandler(rw http.ResponseWriter, req *http.Request, err error) {
	log.Errorf("%s", err.Error())
	rw.WriteHeader(http.StatusBadGateway)
	rw.Write([]byte(err.Error()))
}
