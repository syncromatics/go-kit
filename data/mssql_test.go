package data_test

import (
	"testing"

	"github.com/syncromatics/go-kit/data"

	"github.com/syncromatics/go-kit/testing/docker"
)

func TestShouldConnectToMSSqlDatabase(t *testing.T) {
	testName := "gokit_mssql"
	image := "mcr.microsoft.com/mssql/server:2019-latest"
	db := "db"
	setup := docker.DatabaseSetup{
		TestName:      testName,
		DatabaseImage: &image,
		UserName:      "admin",
		Password:      "admin",
		DatabaseName:  &db,
	}
	settings, err := docker.SetupAdminDatabase(setup)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.TeardownDatabase(testName)

	mssqlSettings := &data.MSSqlSettings{
		Host:     settings.Host,
		Name:     *settings.Name,
		Password: settings.Password,
		User:     settings.User,
		AppName:  "test",
	}

	err = mssqlSettings.WaitForDatabaseToBeOnline(60)
	if err != nil {
		t.Fatal(err)
	}
}
