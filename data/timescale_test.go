package data_test

import (
	"testing"

	"github.com/syncromatics/go-kit/testing/docker"

	_ "github.com/syncromatics/go-kit/internal/migrations/statik"
)

func TestTimescaleMigration(t *testing.T) {
	_, settings, err := docker.SetupTimescaleDatabase("goutils_timescale")
	if err != nil {
		t.Fatal(err)
	}

	err = settings.MigrateUpWithStatik("/timescale")
	if err != nil {
		t.Fatal(err)
	}

	docker.TeardownTimescaleDatabase("goutils_timescale")
}
