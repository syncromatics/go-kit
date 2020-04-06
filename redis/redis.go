package redis

import (
	"time"

	goredis "github.com/go-redis/redis"
)

// WaitForRedisToBeOnline will wait for the redis server to be online for the given seconds.
func WaitForRedisToBeOnline(opts *goredis.Options, secondsToWait int) error {
	var err error
	client := goredis.NewClient(opts)
	for i := 0; i < secondsToWait; i++ {
		_, err := client.Ping().Result()
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	return err
}
