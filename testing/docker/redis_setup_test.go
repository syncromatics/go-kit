package docker_test

import (
	"github.com/syncromatics/go-kit/redis"
	"github.com/syncromatics/go-kit/testing/docker"
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