package docker_test

import (
	"github.com/syncromatics/go-kit/testing/docker"
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