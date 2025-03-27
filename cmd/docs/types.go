// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"log"
	"os"

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
	External    bool     `yaml:"external,omitempty"`
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
	Support       []*SupportVersion    `yaml:"versions"`
}

func (c *Conf) getConf() *Conf {
	yamlFile, err := os.ReadFile("../../documentation/doc.yaml")
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}
	err = yaml.Unmarshal(yamlFile, c) //nolint:musttag
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}

	yamlFile, err = os.ReadFile("../../documentation/lifecycle.yaml")
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}
	support := Support{}
	err = yaml.Unmarshal(yamlFile, &support)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}
	c.Support = support.Versions

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
