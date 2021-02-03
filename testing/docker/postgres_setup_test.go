package docker_test

import (
	"github.com/syncromatics/go-kit/v2/testing/docker"
)

func ExampleSetupPostgresDatabase() {
	db, _, err := docker.SetupPostgresDatabase("test")
	if err != nil {
		panic(err)
	}

	err = db.Ping()
	if err != nil {
		panic(err)
	}

	err = db.Close()
	if err != nil {
		panic(err)
	}

	docker.TeardownPostgresDatabase("test")
	// Output:
}

func ExampleSetupTimescaleDatabase() {
	db, _, err := docker.SetupTimescaleDatabase("test")
	if err != nil {
		panic(err)
	}

	err = db.Ping()
	if err != nil {
		panic(err)
	}

	err = db.Close()
	if err != nil {
		panic(err)
	}

	docker.TeardownTimescaleDatabase("test")
	// Output:
}
