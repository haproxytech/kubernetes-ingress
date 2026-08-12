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

package ingress

import (
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
	"github.com/haproxytech/kubernetes-ingress/pkg/route"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// routeKeyCollision is logged when two ingresses declare the same routing key with different
// values. Split over several lines because revive caps source lines at 200 characters.
//
// The two declarations are named winner first, never "the first one declared": the ingresses
// are walked in map order, so which one is met first varies from one reconciliation to the
// next, and a message whose wording changed while describing the same collision would read as
// a new event every time. Ordering by value costs nothing and is what an operator needs anyway
// - the value in effect is the answer, not the declaration order.
const routeKeyCollision = "routing key '%s' of map '%s' is declared with two different values: '%s' by ingress " +
	"'%s', and '%s' by ingress '%s'. A key has a single answer and haproxy keeps the first matching row of the map " +
	"file, which is the lowest value once the rows are sorted, so only '%s' is ever used - the other value is " +
	"unreachable, and so is any frontend rule whose id it carries. Give the two ingresses distinct hosts or paths"

// routeKeyCollisionSelf is the same collision inside a single ingress, which is what happens
// as soon as a map key does not hold the path.
const routeKeyCollisionSelf = "routing key '%s' of map '%s' is declared by ingress '%s' with two different values, " +
	"'%s' and '%s'. A key has a single answer and haproxy keeps the first matching row of the map file, which is the " +
	"lowest value once the rows are sorted, so only '%s' is ever used. Two paths of one host collapse onto the same " +
	"key whenever the map does not hold the path, which is the case of the sni map serving ssl-passthrough: those " +
	"paths cannot be routed separately, they need distinct hosts"

// reportRouteKeyCollisions records the routing keys this route declares and reports those
// another ingress has already declared with a different value.
//
// A backend is a shared resource, several ingresses referencing the same service port is
// normal. A routing key is not: haproxy resolves a request to one value, the first matching
// row of the map file, and the rows are sorted by content, so the winner is the lowest value
// - which is arbitrary from the operator's point of view. Nothing in the generated
// configuration says the other row exists, which is why this is worth a warning even though
// the routing itself is left untouched here.
//
// Two declarations of the same key with the *same* value are not reported above debug level:
// the rows are identical, so which one haproxy keeps makes no difference. host.map is the
// systematic case, its value being the host itself, which is why every ingress of a host does
// not produce a warning.
//
// A single ingress can collide with itself, and that is not a false positive: as soon as a map
// key does not hold the path - the sni map, keyed on the host alone - two paths of one host
// land on the same key. That is the layer 4 granularity limit, reported here for the first
// time rather than silently losing a path.
//
// Custom routes are not covered: route.AddCustomRoute emits use_backend rules rather than map
// rows, so its collisions are a different mechanism, on the rule order.
func (i *Ingress) reportRouteKeyCollisions(k store.K8s, ingRoute route.Route) {
	rows, err := route.MapRows(ingRoute)
	if err != nil {
		// AddHostPathRoute reports it, and writes nothing: no key is claimed.
		return
	}
	for _, row := range rows {
		declaredKeys := k.RoutesProcessedByMapFile[string(row.Map)]
		if declaredKeys == nil {
			declaredKeys = map[string]store.RouteOwner{}
			k.RoutesProcessedByMapFile[string(row.Map)] = declaredKeys
		}
		owner, declared := declaredKeys[row.Key]
		if !declared {
			declaredKeys[row.Key] = store.RouteOwner{Ingress: i.fqn(), Value: row.Value}
			continue
		}
		if owner.Value == row.Value {
			logger.Debugf("routing key '%s' of map '%s' is declared more than once with the same value '%s'",
				row.Key, row.Map, row.Value)
			continue
		}
		// Which of the two haproxy serves is not decided by which one was declared first: the
		// rows are sorted before the file is written, so the insertion order is erased, and
		// haproxy answers with the first matching row. The one already recorded here is merely
		// the one the walk met first, which decides nothing.
		//
		// The two rows are therefore compared with maps.Compare, the very order the file is
		// written in, and on the rows themselves rather than on the values: if the row format
		// or that order ever change, this follows instead of describing an order the file no
		// longer has. Naming the winner first also keeps the message identical from one
		// reconciliation to the next, the walk being over a map.
		ownerRow := route.Row{Map: row.Map, Key: row.Key, Value: owner.Value}
		winner, loser := owner, store.RouteOwner{Ingress: i.fqn(), Value: row.Value}
		if maps.Compare(row.Line(), ownerRow.Line()) < 0 {
			winner, loser = loser, winner
		}
		if owner.Ingress == i.fqn() {
			logger.Warningf(routeKeyCollisionSelf, row.Key, row.Map, i.fqn(),
				winner.Value, loser.Value, winner.Value)
			continue
		}
		logger.Warningf(routeKeyCollision, row.Key, row.Map,
			winner.Value, winner.Ingress, loser.Value, loser.Ingress, winner.Value)
	}
}

// fqn returns the namespace/name identity of the ingress, as used in logs.
func (i *Ingress) fqn() string {
	return i.resource.Namespace + "/" + i.resource.Name
}
