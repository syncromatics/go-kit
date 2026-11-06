package docker_test

import (
	"fmt"

	"github.com/syncromatics/go-kit/v2/testing/docker"
)

func ExampleSetupEtcd() {
	etcdURL, err := docker.SetupEtcd("test")
	if err != nil {
		panic(err)
	}

	fmt.Printf("etcdURL is set: %t\n", etcdURL != "")
	/*
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{etcdURL},
			DialTimeout: 5*time.Second,
		})
		if err != nil {
			panic(err)
		}

		// snip

		err = cli.Close()
		if err != nil {
			panic(err)
		}
	*/

	docker.TeardownEtcd("test")
	// Output: etcdURL is set: true
}
