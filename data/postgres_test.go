package data_test

import (
	"testing"

	"github.com/syncromatics/go-kit/testing/docker"

	_ "github.com/syncromatics/go-kit/internal/migrations/statik"
)

func TestPostgresMigration(t *testing.T) {
	_, settings, err := docker.SetupPostgresDatabase("goutils_postgres")
	if err != nil {
		t.Fatal(err)
	}

	err = settings.MigrateUpWithStatik("/postgres")
	if err != nil {
		t.Fatal(err)
	}

	docker.TeardownPostgresDatabase("goutils_postgres")
}
