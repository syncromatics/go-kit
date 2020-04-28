package docker_test

import (
	"github.com/syncromatics/go-kit/testing/docker"
)

func ExampleSetupMSSqlDatabase() {
	settings, err := docker.SetupMSSqlDatabase(docker.DatabaseSetup{
		TestName: "test",
		UserName: "sa",
		Password: "superS3cureP@ssword", // The Microsoft SQL Server image has password complexity requirements by default
	})
	if err != nil {
		panic(err)
	}
	
	db, err := settings.GetDB()
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

	docker.TeardownMSSqlDatabase("test")
	// Output: 
}