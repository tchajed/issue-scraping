// vim: ts=4:sw=4
package issues

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
)

type Tracker interface {
	GetAll() *Database
}

func GetJson(baseURL string,
	params map[string]string) (v map[string]interface{}, err error) {
	p := url.Values{}
	for key, val := range params {
		p.Add(key, val)
	}
	resp, err := http.Get(baseURL + "?" + p.Encode())
	if err != nil {
		return
	}
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&v)
	return
}

type Id string

type Issue struct {
	Id
	Title    string
	Name     string // eg, "#53" for github and "YARN-499" for JIRA
	Body     string
	Comments []Comment
}

func trim(s string, length int) string {
	if len(s) > length {
		return s[:length-3] + "..."
	}
	return s
}

func (iss Issue) String() string {
	return fmt.Sprintf("Issue[Id=%s, Title=%s, Body=%s, Comments=%v]",
		iss.Id,
		trim(iss.Title, 30),
		trim(iss.Body, 30),
		iss.Comments)
}

type Comment struct {
	AuthorName  string
	AuthorEmail string
	Body        string
}

func (c Comment) String() string {
	return fmt.Sprintf("[%s <%s> %s]",
		c.AuthorName,
		c.AuthorEmail,
		trim(c.Body, 30),
	)
}

// Database of discovered issues and dependency relationships among them.
// Maintains a tree for issues organized in a DAG as well as a more general
// undirected graph (in the form of an adjacency list). Safe to access from
// multiple goroutines.
type Database struct {
	Issues map[Id]Issue
	Tree   map[Id]Id   // map issues to their parents
	Graph  map[Id][]Id // undirected graph of relationships among issues
	m      *sync.Mutex
}

func NewDatabase() *Database {
	return &Database{
		Issues: make(map[Id]Issue),
		Tree:   make(map[Id]Id),
		Graph:  make(map[Id][]Id),
		m:      &sync.Mutex{},
	}
}

func (db *Database) AddIssue(iss Issue) {
	db.m.Lock()
	defer db.m.Unlock()
	db.Issues[iss.Id] = iss
}

func (db *Database) SetParent(iss, parent Id) {
	db.m.Lock()
	defer db.m.Unlock()
	db.Tree[iss] = parent
}

func (db *Database) AddRelation(a, b Id) {
	db.m.Lock()
	defer db.m.Unlock()
	db.Graph[a] = append(db.Graph[a], b)
	if a != b {
		db.Graph[b] = append(db.Graph[b], a)
	}
}
