package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/pkg/errors"
)

// PostgresDatabaseSettings are the settings for a postgres database
type PostgresDatabaseSettings struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
}

// WaitForDatabaseToBeOnline will wait for the database server to be online for the given seconds.
func (ds *PostgresDatabaseSettings) WaitForDatabaseToBeOnline(secondsToWait int) error {
	db, err := sql.Open("postgres", ds.getConnectionStringWithoutDatabase())
	if err != nil {
		return err
	}
	defer db.Close()

	for i := 0; i < secondsToWait; i++ {
		err = db.Ping()
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	return err
}

// EnsureDatabaseExistsAndGetConnection will create the database if it doesn't exist and return a connection.
func (ds *PostgresDatabaseSettings) EnsureDatabaseExistsAndGetConnection() (*sql.DB, error) {
	db, err := sql.Open("postgres", ds.getConnectionStringWithoutDatabase())
	if err != nil {
		return nil, err
	}
	defer db.Close()

	r := 0
	row := db.QueryRow("select 1 from pg_database where datname=$1", ds.Name)
	err = row.Scan(&r)

	if err != nil && err.Error() != "sql: no rows in result set" {
		return nil, err
	}

	if r != 1 {
		_, err = db.Exec(fmt.Sprintf("create database %s;", ds.Name))
		if err != nil {
			return nil, err
		}
	}

	innerDb, err := sql.Open("postgres", ds.getConnectionStringWithDatabase())
	if err != nil {
		return nil, err
	}

	return innerDb, nil
}

// MigrateUpWithStatik migrates the database using statik
func (ds *PostgresDatabaseSettings) MigrateUpWithStatik(subdirectory string) error {
	source.Register("statik", &statikReader{})

	db, err := ds.EnsureDatabaseExistsAndGetConnection()
	if err != nil {
		return errors.Wrap(err, "failed connecting to db for migration")
	}
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return err
	}

	sourceURL := fmt.Sprintf("statik://%s", subdirectory)

	m, err := migrate.NewWithDatabaseInstance(sourceURL, "postgres", driver)
	if err != nil {
		return err
	}

	err = m.Up()
	if err != nil && err.Error() != "no change" {
		return err
	}

	return nil
}

func (ds *PostgresDatabaseSettings) getPort() int {
	if ds.Port != 0 {
		return ds.Port
	}

	return 5432
}

func (ds *PostgresDatabaseSettings) getConnectionStringWithoutDatabase() string {
	return fmt.Sprintf("user=%s password=%s host=%s port=%d sslmode=disable", ds.User, ds.Password, ds.Host, ds.getPort())
}

func (ds *PostgresDatabaseSettings) getConnectionStringWithDatabase() string {
	return fmt.Sprintf("user=%s password=%s host=%s port=%d dbname=%s sslmode=disable", ds.User, ds.Password, ds.Host, ds.getPort(), ds.Name)
}
