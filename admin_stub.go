//go:build !admin

package main

import (
	"database/sql"
	"net/http"
)

// registerAdmin is a no-op in the public build. The private admin build
// (admin.go, -tags admin) replaces it with maintenance management routes.
func registerAdmin(_ *http.ServeMux, _ *sql.DB, _ Config) {}
