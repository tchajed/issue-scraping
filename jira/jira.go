// vim: ts=4:sw=4
package jira

import (
	"fmt"
	"issues"
	"sync"
)

const InitialMaxResults int = 200

// synchronized set of strings
type stringSet struct {
	Set map[string]bool
	m   *sync.RWMutex
}

func (s stringSet) Contains(str string) bool {
	s.m.RLock()
	defer s.m.RUnlock()
	return s.Set[str]
}

func (s stringSet) Add(str string) {
	s.m.Lock()
	defer s.m.Unlock()
	s.Set[str] = true
}

// Represents a JIRA issue tracking instance
type Tracker struct {
	baseURL    string
	total      int
	maxResults int
	DB         *issues.Database
	issueLinks stringSet // set of issue links (by link id) scraped
}

func (t *Tracker) url(path string) string {
	return t.baseURL + "/rest/api/latest" + path
}

func (t *Tracker) Search(start int) (params map[string]string) {
	params = make(map[string]string)
	params["jql"] = "ORDER BY Created Asc"
	params["startAt"] = fmt.Sprintf("%d", start)
	params["maxResults"] = fmt.Sprintf("%d", t.maxResults)
	return params
}

func NewTracker(url string) (t *Tracker) {
	t = &Tracker{
		baseURL:    url,
		maxResults: InitialMaxResults,
		DB:         issues.NewDatabase(),
		issueLinks: stringSet{
			Set: make(map[string]bool),
			m:   &sync.RWMutex{}},
	}
	return
}

func (t *Tracker) AddIssueLink(from issues.Id, link map[string]interface{}) {
	id := link["id"].(string)
	// if already processed, ignore
	if t.issueLinks.Contains(id) {
		return
	}
	if _, ok := link["inwardIssue"]; ok {
		other := getmap(link["inwardIssue"])
		t.DB.AddRelation(from, toId(other["id"]))
	}
	t.issueLinks.Add(id)
}

// Helper for working with JSON objects: type-asserts interface to JSON object.
// If provided map is nil, returns a new map (which can be safely indexed for
// zero values).
func getmap(v interface{}) map[string]interface{} {
	if v == nil {
		return make(map[string]interface{})
	}
	return v.(map[string]interface{})
}

// Helper for working with JSON objects: type asserts interface to issues.Id
func toId(v interface{}) issues.Id {
	return issues.Id(v.(string))
}

// Safely get a string value from a map
func getstring(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok && v != nil {
		return v.(string)
	}
	return ""
}

// Fetch all issues from JIRA with a concurrency of N parallel fetches.
func (t *Tracker) FetchAll(N int) {
	firstBatchEnd := t.GetFrom(0)
	// check if the first search returned all the results
	if firstBatchEnd >= t.total {
		return
	}
	work := make(chan int)
	done := make(chan bool)
	for i := 0; i < N; i++ {
		go func() {
			for start := range work {
				t.GetFrom(start)
				t.PrintParams()
			}
			done <- true
		}()
	}
	for start := firstBatchEnd; start < t.total/2; start += t.maxResults {
		work <- start
	}
	close(work)
	for i := 0; i < N; i++ {
		<-done
	}
}

func (t *Tracker) GetAll() *issues.Database {
	return t.DB
}

// Get issues starting from a particular search result number. Returns the
// number of the last result found.
func (t *Tracker) GetFrom(start int) int {
	db := t.DB
	params := t.Search(start)
	params["fields"] = "id,summary,description,comment,parent,issuelinks"
	r, err := issues.GetJson(t.url("/search"), params)
	if err != nil {
		return start
	}
	if _, ok := r["maxResults"]; ok {
		t.maxResults = int(r["maxResults"].(float64))
	}
	if t.total == 0 {
		t.total = int(r["total"].(float64))
	}
	issueList := r["issues"].([]interface{})
	for _, issueInterface := range issueList {
		issueMap := getmap(issueInterface)
		issue := issues.Issue{}
		issue.Id = toId(issueMap["id"])
		issue.Name = getstring(issueMap, "key")

		// Base fields
		fields := getmap(issueMap["fields"])
		issue.Title = getstring(fields, "summary")
		issue.Body = getstring(fields, "description")

		// Comments
		commentInfo := getmap(fields["comment"])
		issue.Comments = make([]issues.Comment, 0,
			int(commentInfo["maxResults"].(float64)))
		comments := commentInfo["comments"].([]interface{})
		for _, commentInterface := range comments {
			comment := issues.Comment{}
			commentMap := getmap(commentInterface)
			comment.Body = getstring(commentMap, "body")
			author := getmap(commentMap["author"])
			comment.AuthorName = getstring(author, "displayName")
			comment.AuthorEmail = getstring(author, "emailAddress")
			issue.Comments = append(issue.Comments, comment)
		}

		db.AddIssue(issue)

		// Links
		// parent links
		if _, ok := fields["parent"]; ok {
			parentInfo := getmap(fields["parent"])
			db.SetParent(issue.Id, toId(parentInfo["id"]))
		}

		// general links
		for _, issueLinkInterface := range fields["issuelinks"].([]interface{}) {
			link := getmap(issueLinkInterface)
			t.AddIssueLink(issue.Id, link)
		}
	}
	return start + len(issueList)
}

// For debugging purposes
func (t *Tracker) PrintParams() {
	fmt.Printf("finished: %d total: %d maxResults: %d\n",
		len(t.DB.Issues),
		t.total,
		t.maxResults)
}
