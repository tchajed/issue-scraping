// vim: ts=4:sw=4
package issues

import (
	"fmt"
	"sync"
	"time"
)

type Tracker interface {
	GetAll() *Database
}

type Id string

type Link struct {
	From    Id
	To      Id
	Type    string
	Created time.Time
}

// Helper for working with JSON objects: type asserts interface to Id
func ToId(v interface{}) Id {
	return Id(v.(string))
}

type Issue struct {
	Id
	Title    string
	Created  time.Time
	Name     string // eg, "#53" for github and "YARN-499" for JIRA
	Body     string
	Comments []Comment
}

// internal function to shorten string representation of potentially large
// issues/comments
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
	Created     time.Time
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
	Tree   map[Id]Id     // map issues to their parents
	Graph  map[Id][]Link // directed graph of links among issues
	m      *sync.Mutex
}

func NewDatabase() *Database {
	return &Database{
		Issues: make(map[Id]Issue),
		Tree:   make(map[Id]Id),
		Graph:  make(map[Id][]Link),
		m:      &sync.Mutex{},
	}
}

func (db *Database) AddIssue(iss Issue) {
	db.m.Lock()
	defer db.m.Unlock()
	db.Issues[iss.Id] = iss
}

// Add an edge to the tree part of the database
func (db *Database) SetParent(iss, parent Id) {
	db.m.Lock()
	defer db.m.Unlock()
	db.Tree[iss] = parent
}

// Add a directed relationship to the general directed graph of the
// database. Self-loops are allowed, but uniqueness of the edge is not
// checked.
func (db *Database) AddLink(l Link) {
	db.m.Lock()
	defer db.m.Unlock()
	db.Graph[l.From] = append(db.Graph[l.From], l)
}
