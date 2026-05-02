// Copyright (c) 2026 VitruvianSoftware
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Package database defines supported database engines and their container
// configurations for one-click local provisioning via devx db spawn.
package database

import "fmt"

// Engine holds the container configuration for a database engine.
type Engine struct {
	Name          string            // Human-readable name
	Image         string            // Container image
	DefaultPort   int               // Default port on the host
	InternalPort  int               // Port inside the container
	VolumePath    string            // Data directory inside the container
	Env           map[string]string // Default environment variables
	ReadyLog      string            // Log line that indicates the DB is ready
	ConnStringFmt string            // printf format for the connection string: host, port, user, pass, dbname
}

// Registry of all supported database engines.
var Registry = map[string]Engine{
	"postgres": {
		Name:         "PostgreSQL",
		Image:        "docker.io/library/postgres:16-alpine",
		DefaultPort:  5432,
		InternalPort: 5432,
		VolumePath:   "/var/lib/postgresql/data",
		Env: map[string]string{
			"POSTGRES_USER":     "devx",
			"POSTGRES_PASSWORD": "devx",
			"POSTGRES_DB":       "devx",
		},
		ReadyLog:      "database system is ready to accept connections",
		ConnStringFmt: "postgresql://%s:%s@localhost:%d/%s",
	},
	"redis": {
		Name:          "Redis",
		Image:         "docker.io/library/redis:7-alpine",
		DefaultPort:   6379,
		InternalPort:  6379,
		VolumePath:    "/data",
		Env:           map[string]string{},
		ReadyLog:      "Ready to accept connections",
		ConnStringFmt: "redis://localhost:%d",
	},
	"mysql": {
		Name:         "MySQL",
		Image:        "docker.io/library/mysql:8",
		DefaultPort:  3306,
		InternalPort: 3306,
		VolumePath:   "/var/lib/mysql",
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": "devx",
			"MYSQL_DATABASE":      "devx",
			"MYSQL_USER":          "devx",
			"MYSQL_PASSWORD":      "devx",
		},
		ReadyLog:      "ready for connections",
		ConnStringFmt: "mysql://%s:%s@localhost:%d/%s",
	},
	"mongo": {
		Name:         "MongoDB",
		Image:        "docker.io/library/mongo:7",
		DefaultPort:  27017,
		InternalPort: 27017,
		VolumePath:   "/data/db",
		Env: map[string]string{
			"MONGO_INITDB_ROOT_USERNAME": "devx",
			"MONGO_INITDB_ROOT_PASSWORD": "devx",
		},
		ReadyLog:      "Waiting for connections",
		ConnStringFmt: "mongodb://%s:%s@localhost:%d",
	},
}

// ConnString returns the formatted connection string for a given engine.
func (e Engine) ConnString(port int) string {
	switch e.Name {
	case "Redis":
		return fmt.Sprintf("redis://localhost:%d", port)
	case "MongoDB":
		user := e.Env["MONGO_INITDB_ROOT_USERNAME"]
		pass := e.Env["MONGO_INITDB_ROOT_PASSWORD"]
		return fmt.Sprintf(e.ConnStringFmt, user, pass, port)
	case "PostgreSQL":
		user := e.Env["POSTGRES_USER"]
		pass := e.Env["POSTGRES_PASSWORD"]
		db := e.Env["POSTGRES_DB"]
		return fmt.Sprintf(e.ConnStringFmt, user, pass, port, db)
	case "MySQL":
		user := e.Env["MYSQL_USER"]
		pass := e.Env["MYSQL_PASSWORD"]
		db := e.Env["MYSQL_DATABASE"]
		return fmt.Sprintf(e.ConnStringFmt, user, pass, port, db)
	default:
		return fmt.Sprintf("localhost:%d", port)
	}
}

// SupportedEngines returns a sorted list of engine names.
func SupportedEngines() []string {
	return []string{"postgres", "redis", "mysql", "mongo"}
}
