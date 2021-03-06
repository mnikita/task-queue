package util

import (
	"github.com/fsnotify/fsnotify"
	"github.com/mnikita/task-queue/pkg/log"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"
)

type ConfigWatcherEventHandler interface {
	OnConfigModified()
}

type ConfigWatcher struct {
	eventHandler ConfigWatcherEventHandler

	watcher *fsnotify.Watcher

	watchStarted bool
}

func GetSystemConcurrency() (concurrency int) {
	return runtime.GOMAXPROCS(0)
}

func WaitTimeout(wg *sync.WaitGroup, timeout time.Duration) error {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return nil // completed normally
	case <-time.After(timeout):
		return log.WorkerWaitTimeoutError(timeout) // timed out
	}
}

func IsNil(i interface{}) bool {
	if i == nil {
		return true
	}
	switch reflect.TypeOf(i).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
		return reflect.ValueOf(i).IsNil()
	}
	return false
}

func CheckEnvForValue(name string, v string) string {
	if v == "" {
		v, _ = os.LookupEnv(name)
	}

	return v
}

func CheckEnvForArray(name string, a []string) []string {
	if a == nil || len(a) == 0 {
		t, ok := os.LookupEnv(name)

		if ok && t != "" {
			a = strings.Split(t, ",")

			for i := range a {
				a[i] = strings.TrimSpace(a[i])
			}
		}
	}

	return a
}

func NewConfigWatcher(eventHandler ConfigWatcherEventHandler) (c *ConfigWatcher, err error) {
	c = &ConfigWatcher{}

	c.eventHandler = eventHandler

	// creates a new file watcher
	c.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *ConfigWatcher) WatchConfigFile(configFile string) (err error) {
	if !c.watchStarted {
		go func() {
			log.Logger().ConfigWatchStart()

			for {
				select {
				case event, ok := <-c.watcher.Events:
					if !ok {
						return
					}

					if event.Op&fsnotify.Write == fsnotify.Write {
						log.Logger().ConfigWatchModified(event.Name)

						c.eventHandler.OnConfigModified()
					}
				case err, ok := <-c.watcher.Errors:
					if !ok {
						return
					}

					log.Logger().ConfigWatchError(err)
				}
			}
		}()
	}

	if configFile != "" {
		err = c.watcher.Add(configFile)
		if err != nil {
			return err
		}

		log.Logger().ConfigWatchFile(configFile)
	}

	return nil
}

func (c *ConfigWatcher) StopWatch() (err error) {
	log.Logger().ConfigWatchStop()

	return c.watcher.Close()
}
