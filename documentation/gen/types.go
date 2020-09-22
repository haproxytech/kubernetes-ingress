package main

import (
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v3"
)

type ConfArg struct {
	Argument    string   `yaml:"argument"`
	Default     string   `yaml:"default"`
	Description string   `yaml:"description,flow"`
	Tip         []string `yaml:"tip"`
	Values      []string `yaml:"values"`
	VersionMin  Version  `yaml:"version_min"`
	VersionMax  Version  `yaml:"version_max,omitempty"`
	Example     string   `yaml:"example,omitempty,flow"`
	Helm        string   `yaml:"helm,omitempty"`
}

type ConfItem struct {
	Title            string   `yaml:"title"`
	Type             string   `yaml:"type"`
	Group            string   `yaml:"group"`
	Dependencies     string   `yaml:"dependencies"`
	Default          string   `yaml:"default"`
	Description      []string `yaml:"description"`
	Tip              []string `yaml:"tip"`
	Values           []string `yaml:"values"`
	AppliesTo        []string `yaml:"applies_to"`
	VersionMin       Version  `yaml:"version_min"`
	VersionMax       Version  `yaml:"version_max,omitempty"`
	Example          []string `yaml:"example,omitempty,flow"`
	ExampleConfigmap string   `yaml:"example_configmap,omitempty,flow"`
	ExampleIngress   string   `yaml:"example_ingress,omitempty,flow"`
	ExampleService   string   `yaml:"example_service,omitempty,flow"`
}

type ConfGroup struct {
	Group       string   `yaml:"group"`
	Description []string `yaml:"description"`
	Header      string   `yaml:"header,omitempty"`
	Footer      string   `yaml:"footer,omitempty"`
}

type Conf struct {
	ActiveVersion Version              `yaml:"active_version"`
	Arguments     []*ConfArg           `yaml:"image_arguments"`
	Groups        map[string]ConfGroup `yaml:"groups"`
	Items         []*ConfItem          `yaml:"annotations"`
}

func (c *Conf) getConf() *Conf {

	yamlFile, err := ioutil.ReadFile("../doc.yaml")
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}

	return c
}

func Contains(values []string, key string) string {
	for _, v := range values {
		if v == key {
			return ":large_blue_circle:"
		}
	}
	return ":white_circle:"
}
func ContainsB(values []string, key string) bool {
	for _, v := range values {
		if v == key {
			return true
		}
	}
	return false
}
