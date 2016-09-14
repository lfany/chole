package main

import (
	"fmt"
	"io/ioutil"
	"strconv"

	"github.com/go-fsnotify/fsnotify"
	"gopkg.in/yaml.v2"
)

type Rule struct {
	Out string `yaml:"out"`
	In  string `yaml:"in"`
}

type Config struct {
	Server string           `yaml:"server"`
	Rules  map[string]*Rule `yaml:"rules"`
}

var clients map[string]Client

func (rule Rule) getId(name string) string {
	return fmt.Sprintf("%s:%s:%s", name, rule.In, rule.Out)
}

func (config *Config) parse() {
	content, err := ioutil.ReadFile("./config.yaml")
	if err != nil {
		Fatal("CONFIG", err)
	}
	if err = yaml.Unmarshal(content, &config); err != nil {
		Fatal("CONFIG", err)
	}
	if config.Server == "" {
		config.Server = "localhost"
	}
	for _, rule := range config.Rules {
		if rule.In == "" {
			Fatal("CONFIG", "must specify 'in' port in rule")
		}
		if rule.Out == "" {
			rule.Out = rule.In
		}
		if _, err := strconv.Atoi(rule.In); err == nil {
			rule.In = ":" + rule.In
		}
	}
}

func (config *Config) apply() {
	config.parse()

	idMap := make(map[string]bool)
	for name, rule := range config.Rules {
		id := rule.getId(name)
		idMap[id] = true
	}

	for id, client := range clients {
		if _, ok := idMap[id]; !ok {
			client.Close()
			delete(clients, id)
		}
	}

	for name, rule := range config.Rules {
		id := rule.getId(name)
		if _, ok := clients[id]; !ok {
			client := Client{
				server: config.Server,
				name:   name,
				in:     rule.In,
				out:    rule.Out,
			}
			client.Start()
			clients[id] = client
		}
	}
}

func (config *Config) Watch() {
	clients = make(map[string]Client)
	config.apply()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		Error("CONFIG", err)
		return
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if event.Op&fsnotify.Write == fsnotify.Write {
					config.apply()
					Log("CONFIG", "applyed new configuration")
				}
			case err := <-watcher.Errors:
				Error("CONFIG", err)
			}
		}
	}()

	err = watcher.Add("./config.yaml")
	if err != nil {
		Fatal("CONFIG", err)
	}
	<-done
}
