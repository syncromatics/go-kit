package docker_test

import (
	"github.com/syncromatics/go-kit/v2/redis"
	"github.com/syncromatics/go-kit/v2/testing/docker"
)

func ExampleSetupRedis() {
	options, err := docker.SetupRedis("test")
	if err != nil {
		panic(err)
	}

	err = redis.WaitForRedisToBeOnline(options, 10)
	if err != nil {
		panic(err)
	}

	docker.TeardownRedis("test")
	// Output:
}
