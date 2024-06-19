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

package store

func (k *K8s) EventTCPCR(namespace, name string, data *TCPs) bool {
	ns := k.GetNamespace(namespace)

	updateRequired := false
	switch data.Status {
	case MODIFIED:
		newTCP := data
		oldTCP, ok := ns.CRs.TCPsPerCR[data.Name]
		if !ok {
			// It can happen (resync) that we receive an UPDATE on a item that is not yet registered
			// We should treat it as a CREATE.
			logger.Warningf("TCP CR '%s' not registered with controller !", data.Name)
			data.Status = ADDED
			return k.EventTCPCR(namespace, name, data)
		}
		// No change detected, do nothing
		if oldTCP.Equal(newTCP) {
			return updateRequired
		}

		// There is some change
		k.handleDeletedTCPs(oldTCP, newTCP)

		ns.CRs.TCPsPerCR[data.Name] = newTCP
		ns.CRs.AllTCPs = make(TCPResourceList, 0)
		for _, v := range ns.CRs.TCPsPerCR {
			ns.CRs.AllTCPs = append(ns.CRs.AllTCPs, v.Items...)
		}
		ns.CRs.AllTCPs.Order()
		// Check collisions with TCP in all namespaces
		k.checkCollisionsAllNamespaces()
		updateRequired = true
	case ADDED:
		// ADDED received, but we already have it
		if old, ok := ns.CRs.TCPsPerCR[data.Name]; ok {
			if old.Status == DELETED {
				ns.CRs.TCPsPerCR[data.Name].Status = ADDED
			}
			if !old.Equal(data) {
				data.Status = MODIFIED
				return k.EventTCPCR(namespace, name, data)
			}
			return updateRequired
		}

		// There is a new TCP
		ns.CRs.TCPsPerCR[data.Name] = data
		ns.CRs.AllTCPs = make(TCPResourceList, 0)
		for _, v := range ns.CRs.TCPsPerCR {
			ns.CRs.AllTCPs = append(ns.CRs.AllTCPs, v.Items...)
		}
		ns.CRs.AllTCPs.Order()
		// Check collisions with TCP in all namespaces
		k.checkCollisionsAllNamespaces()
		updateRequired = true
	case DELETED:
		tcp, ok := ns.CRs.TCPsPerCR[data.Name]
		if ok {
			tcp.Status = DELETED
			k.handleDeletedTCPs(tcp, nil)
			delete(ns.CRs.TCPsPerCR, data.Name)
			ns.CRs.AllTCPs = make(TCPResourceList, 0)
			for _, v := range ns.CRs.TCPsPerCR {
				ns.CRs.AllTCPs = append(ns.CRs.AllTCPs, v.Items...)
			}
			ns.CRs.AllTCPs.Order()
			// Check collisions with TCP in all namespaces
			k.checkCollisionsAllNamespaces()
			updateRequired = true
		} else {
			logger.Warningf("TCP CR '%s' not registered with controller, cannot delete !", data.Name)
		}
	}
	return updateRequired
}

func (k *K8s) handleDeletedTCPs(oldTCP, newTCP *TCPs) {
	for _, old := range oldTCP.Items {
		found := false
		if newTCP != nil {
			for _, new := range newTCP.Items {
				if old.Name == new.Name {
					found = true
					break
				}
			}
		}
		if !found {
			owner := old.Owner()
			k.FrontendRC.RemoveOwner(owner)
		}
	}
}

func (k *K8s) checkCollisionsAllNamespaces() {
	tcpAllNamespaces := k.tcpsAllNamespaces()
	tcpAllNamespaces.Order()
	tcpAllNamespaces.CheckCollision()
}

func (k *K8s) tcpsAllNamespaces() TCPResourceList {
	allTCPs := make(TCPResourceList, 0)
	for _, ns := range k.Namespaces {
		for _, v := range ns.CRs.TCPsPerCR {
			allTCPs = append(allTCPs, v.Items...)
		}
	}
	return allTCPs
}
