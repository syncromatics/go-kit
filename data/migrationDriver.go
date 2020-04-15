package data

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/pkg/errors"
	"github.com/rakyll/statik/fs"

	"github.com/golang-migrate/migrate/v4/source"
)

type statikReader struct {
	files     http.FileSystem
	directory string

	migrations *source.Migrations
}

func (b *statikReader) Open(url string) (source.Driver, error) {
	files, err := fs.New()
	if err != nil {
		return nil, errors.Wrap(err, "failed opening fs")
	}

	subdirectory := strings.Replace(url, "statik://", "", -1)

	blob := &statikReader{
		files:      files,
		directory:  subdirectory,
		migrations: source.NewMigrations(),
	}

	walker := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		m, err := source.DefaultParse(info.Name())
		if err != nil {
			return err
		}

		if !blob.migrations.Append(m) {
			return fmt.Errorf("unable to parse file %v", info.Name())
		}

		return nil
	}

	err = fs.Walk(blob.files, subdirectory, walker)
	if err != nil {
		return nil, errors.Wrap(err, "failed walking directory")
	}

	return blob, nil
}

func (b *statikReader) First() (uint, error) {
	v, ok := b.migrations.First()
	if !ok {
		return 0, &os.PathError{Op: "first", Err: os.ErrNotExist}
	}

	return v, nil
}

func (b *statikReader) Prev(version uint) (uint, error) {
	v, ok := b.migrations.Prev(version)
	if !ok {
		return 0, &os.PathError{Op: fmt.Sprintf("prev for version %v", version), Err: os.ErrNotExist}
	}

	return v, nil
}

func (b *statikReader) Next(version uint) (uint, error) {
	v, ok := b.migrations.Next(version)
	if !ok {
		return 0, &os.PathError{Op: fmt.Sprintf("next for version %v", version), Err: os.ErrNotExist}
	}
	return v, nil
}

func (b *statikReader) ReadUp(version uint) (io.ReadCloser, string, error) {
	if m, ok := b.migrations.Up(version); ok {
		r, err := b.files.Open(path.Join(b.directory, m.Raw))
		if err != nil {
			return nil, "", err
		}
		return r, m.Identifier, nil
	}
	return nil, "", &os.PathError{Op: fmt.Sprintf("read version %v", version), Err: os.ErrNotExist}
}

func (b *statikReader) ReadDown(version uint) (io.ReadCloser, string, error) {
	if m, ok := b.migrations.Down(version); ok {
		r, err := b.files.Open(path.Join(b.directory, m.Raw))
		if err != nil {
			return nil, "", err
		}
		return r, m.Identifier, nil
	}
	return nil, "", &os.PathError{Op: fmt.Sprintf("read version %v", version), Err: os.ErrNotExist}
}

func (b *statikReader) Close() error {
	return nil
}
