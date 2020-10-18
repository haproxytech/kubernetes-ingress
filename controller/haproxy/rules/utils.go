package rules

import "fmt"

func makeACL(acl string, mapFile string) (result string) {
	result = fmt.Sprintf("{ var(txn.host),concat(,txn.path) -m beg -f %s }", mapFile) + acl
	result += " or " + fmt.Sprintf("{ var(txn.host) -f %s }", mapFile) + acl
	result += " or " + fmt.Sprintf("{ var(txn.path) -m beg -f %s }", mapFile) + acl
	return result
}
