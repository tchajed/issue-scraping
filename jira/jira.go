// vim: ts=4:sw=4
package jira

import (
	"fmt"
	"issues"
	"jsonutil"
	"sync"
	"time"
)

const InitialMaxResults int = 200

// synchronized set of strings
type stringSet struct {
	Set map[string]bool
	m   *sync.RWMutex
}

func newStringSet() stringSet {
	return stringSet{
		Set: make(map[string]bool),
		m:   &sync.RWMutex{},
	}
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

// The JSON date format used by the JIRA API
const DateFormat = "2006-01-02T15:04:05.000-0700"

func getDate(m map[string]interface{}, fieldname string) time.Time {
	// ignore parse errors (returning UNIX time 0 is sufficient)
	t, _ := time.Parse(DateFormat, jsonutil.GetString(m, fieldname))
	return t
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
		issueLinks: newStringSet(),
	}
	return
}

func (t *Tracker) AddIssueLink(from issues.Id, link map[string]interface{}) {
	id := jsonutil.GetString(link, "id")
	// if already processed, ignore
	if t.issueLinks.Contains(id) {
		return
	}
	if _, ok := link["inwardIssue"]; ok {
		typeInfo := jsonutil.GetMap(link["type"])
		linkType := jsonutil.GetString(typeInfo, "inward")
		other := jsonutil.GetMap(link["inwardIssue"])
		t.DB.AddLink(
			issues.Link{
				From: from,
				To:   issues.ToId(other["id"]),
				Type: linkType,
			},
		)
	}
	t.issueLinks.Add(id)
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
	for start := firstBatchEnd; start < t.total; start += t.maxResults {
		work <- start
	}
	close(work)
	for i := 0; i < N; i++ {
		<-done
	}
}

// Get the database fetched so far.
func (t *Tracker) GetAll() *issues.Database {
	return t.DB
}

func parseComment(commentInterface interface{}) issues.Comment {
	comment := issues.Comment{}
	commentMap := jsonutil.GetMap(commentInterface)
	comment.Created = getDate(commentMap, "created")
	comment.Body = jsonutil.GetString(commentMap, "body")
	author := jsonutil.GetMap(commentMap["author"])
	comment.AuthorName = jsonutil.GetString(author, "displayName")
	comment.AuthorEmail = jsonutil.GetString(author, "emailAddress")
	return comment
}

func parseIssue(issueInterface interface{}) issues.Issue {
	issueMap := jsonutil.GetMap(issueInterface)
	issue := issues.Issue{}
	issue.Id = issues.ToId(issueMap["id"])
	issue.Name = jsonutil.GetString(issueMap, "key")

	// Base fields
	fields := jsonutil.GetMap(issueMap["fields"])
	issue.Created = getDate(fields, "created")
	issue.Title = jsonutil.GetString(fields, "summary")
	issue.Body = jsonutil.GetString(fields, "description")

	// Comments
	commentInfo := jsonutil.GetMap(fields["comment"])
	issue.Comments = make([]issues.Comment, 0,
		int(commentInfo["maxResults"].(float64)))
	comments := commentInfo["comments"].([]interface{})
	for _, commentInterface := range comments {
		comment := parseComment(commentInterface)
		issue.Comments = append(issue.Comments, comment)
	}
	return issue
}

// Get issues starting from a particular search result number. Returns the
// number of the last result found.
func (t *Tracker) GetFrom(start int) int {
	params := t.Search(start)
	// filter the list of fields -- only affects the fields map; in particular,
	// id, key and self (a URL for the issue resource) are always returned
	params["fields"] =
		"summary,description,comment,parent,issuelinks,created"
	params["expand"] = "changelog"
	r, err := jsonutil.Get(t.url("/search"), params)
	if err != nil {
		return start
	}
	if _, ok := r["maxResults"]; ok {
		t.maxResults = int(r["maxResults"].(float64))
	}
	if t.total == 0 {
		t.total = int(r["total"].(float64))
	}
	db := t.DB
	issueList := r["issues"].([]interface{})
	for _, issueInterface := range issueList {
		issue := parseIssue(issueInterface)
		db.AddIssue(issue)

		// Links
		issueMap := jsonutil.GetMap(issueInterface)
		fields := jsonutil.GetMap(issueMap["fields"])

		// parent links
		if _, ok := fields["parent"]; ok {
			parentInfo := jsonutil.GetMap(fields["parent"])
			db.SetParent(issue.Id, issues.ToId(parentInfo["id"]))
		}

		// general links
		for _, issueLinkInterface := range fields["issuelinks"].([]interface{}) {
			link := jsonutil.GetMap(issueLinkInterface)
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
