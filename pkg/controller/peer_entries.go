// Copyright 2026 HAProxy Technologies LLC
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

package controller

import (
	"strings"

	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
)

func (c *HAProxyController) reconcileLocalPeerEntries() error {
	currentEntries, err := c.haproxy.PeerEntriesGet(localPeerSection)
	if err != nil {
		return err
	}

	currentByName := map[string]*models.PeerEntry{}
	for _, currentEntry := range currentEntries {
		if currentEntry != nil {
			currentByName[currentEntry.Name] = currentEntry
		}
	}

	expectedEntries := c.localPeerEntries()
	changed := false
	for _, currentEntry := range currentEntries {
		if currentEntry == nil || !c.managesPeerEntry(currentEntry.Name) {
			continue
		}
		if _, ok := expectedEntries[currentEntry.Name]; ok {
			continue
		}
		if err := c.haproxy.PeerEntryDelete(localPeerSection, currentEntry.Name); err != nil {
			return err
		}
		changed = true
	}

	for name, expectedEntry := range expectedEntries {
		currentEntry, ok := currentByName[name]
		if ok && peerEntryEqual(currentEntry, expectedEntry) {
			continue
		}
		if err := c.haproxy.PeerEntryCreateOrEdit(localPeerSection, expectedEntry); err != nil {
			return err
		}
		changed = true
	}

	if changed {
		instance.Reload("HAProxy local peer entries changed")
	}
	return nil
}

func (c *HAProxyController) localPeerEntries() map[string]models.PeerEntry {
	expectedEntries := map[string]models.PeerEntry{}
	if c.PodIP != "" {
		expectedEntries[c.Hostname] = c.localPeerEntry(c.Hostname, c.PodIP)
	}
	for podName, podIP := range c.store.HaProxyPods {
		if podIP == "" {
			continue
		}
		expectedEntries[podName] = c.localPeerEntry(podName, podIP)
	}
	return expectedEntries
}

func (c *HAProxyController) localPeerEntry(name string, address string) models.PeerEntry {
	return models.PeerEntry{
		Name:    name,
		Address: &address,
		Port:    &c.osArgs.LocalPeerPort,
	}
}

func (c *HAProxyController) managesPeerEntry(name string) bool {
	if c.podPrefix == "" {
		return name == c.Hostname
	}
	return name == c.Hostname || strings.HasPrefix(name, c.podPrefix+"-")
}

func peerEntryEqual(currentEntry *models.PeerEntry, expectedEntry models.PeerEntry) bool {
	if currentEntry == nil || currentEntry.Address == nil || expectedEntry.Address == nil || currentEntry.Port == nil || expectedEntry.Port == nil {
		return false
	}
	return currentEntry.Name == expectedEntry.Name && *currentEntry.Address == *expectedEntry.Address && *currentEntry.Port == *expectedEntry.Port
}
