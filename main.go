package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	_ "github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"gopkg.in/yaml.v2"
)

type logger struct {
}

func (logger) Log(args ...any) error {
	log.Println(args...)
	return nil
}

func readDiscoveryConfig(path string) (map[string]discovery.Configs, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c config.Config
	err = yaml.Unmarshal(b, &c)
	if err != nil {
		return nil, err
	}
	res := make(map[string]discovery.Configs, len(c.ScrapeConfigs))
	for _, sc := range c.ScrapeConfigs {
		res[sc.JobName] = sc.ServiceDiscoveryConfigs
	}
	return res, nil
}

func main() {
	// subscribe for shutdown signals
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt)
	signal.Notify(exit, syscall.SIGTERM)
	// run discoverer
	var (
		d      = make(map[string][]*targetgroup.Group)
		m      = discovery.NewManager(context.Background(), logger{})
		c, err = readDiscoveryConfig("/etc/prometheus/prometheus.yml")
	)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		m.ApplyConfig(c)
		m.Run()
	}()
	go func() {
		for k, v := range <-m.SyncCh() {
			var s []*targetgroup.Group
			for _, t := range v {
				if t.Labels["__meta_kubernetes_namespace"] != "kdiscovery" {
					s = append(s, t)
				}
			}
			if len(s) != 0 {
				d[k] = s
			} else {
				delete(d, k)
			}
		}
	}()
	// run http server
	go func() {
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		http.HandleFunc("/readiness", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			res, err := json.Marshal(d)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(fmt.Sprintf("%v", err)))
			} else {
				w.Header().Add("Content-Type", "application/json")
				w.Write(res)
			}
		})
		err := http.ListenAndServe(":8080", nil)
		if err != nil {
			log.Fatal(err)
		}
	}()
	<-exit
	println("bye")
}
