package data

import (
	"database/sql"
	"fmt"
	"net/url"
	"time"

	mssqldb "github.com/denisenkom/go-mssqldb"
	"github.com/pkg/errors"
)

// MSSqlSettings is the settings for a ms sql database
type MSSqlSettings struct {
	Host     string
	User     string
	Password string
	Name     string
	AppName  string
}

// GetDB will return a database instance from the settings
func (ds *MSSqlSettings) GetDB() (*sql.DB, error) {
	var _ = mssqldb.Decimal{}

	query := url.Values{}
	query.Add("app name", ds.AppName)
	query.Add("database", ds.Name)

	u := &url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(ds.User, ds.Password),
		Host:     fmt.Sprintf("%s:%d", ds.Host, 1433),
		RawQuery: query.Encode(),
	}
	db, err := sql.Open("sqlserver", u.String())
	if err != nil {
		return nil, errors.Wrap(err, "open db failed")
	}

	return db, nil
}

// WaitForDatabaseToBeOnline will wait for the database server to be online for the given seconds.
func (ds *MSSqlSettings) WaitForDatabaseToBeOnline(secondsToWait int) error {
	db, err := ds.GetDB()
	if err != nil {
		return err
	}

	defer db.Close()

	for i := 0; i < secondsToWait; i++ {
		err = db.Ping()
		if err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return errors.Wrap(err, "timed out waiting for database")
}
