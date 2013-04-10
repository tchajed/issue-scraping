// vim: ts=4:sw=4
package main

import (
	"fmt"
	"issues/jira"
)

func main() {
	t := jira.NewTracker("https://issues.apache.org/jira")
	t.GetFrom(0)
	t.PrintParams()
	fmt.Println(len(t.DB.Issues))
	fmt.Println(t.DB.Tree)
	fmt.Println(t.DB.Graph)
}
