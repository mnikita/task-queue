//go:generate mockgen -destination=./mocks/mock_cli.go -package=mocks . Handler
package cli

import (
	"encoding/json"
	"github.com/mnikita/task-queue/pkg/common"
	"github.com/mnikita/task-queue/pkg/container"
	"github.com/mnikita/task-queue/pkg/log"
	"github.com/mnikita/task-queue/pkg/util"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const EnvUrl = "BEANSTALKD_URL"
const EnvTubes = "BEANSTALKD_TUBES"

type OsSignalCallback func(chan os.Signal)

type Handler interface {
	Init() error
	Close() error

	//Container() container.Handler

	Start(OsSignalCallback) error
	Put(taskData []byte) (uint64, error)
	Delete(uint64) error
	PutFromFile() (uint64, error)
	WriteDefaultConfiguration(writer io.Writer) (int, error)
	WriteDefaultConfigurationToFile(file string) (int, error)
}

type Configuration struct {
	Tubes        []string
	Url          string
	ConfigFile   string
	TaskDataFile string

	PutPriority uint32
	PutDelay    time.Duration
	PutTtr      time.Duration
}

type Cli struct {
	*Configuration

	container container.Handler
}

func validateConfig(config *Configuration) (err error) {
	if config.Url == "" {
		err = log.MissingCliUrl()
	}

	return err
}

func NewConfiguration() *Configuration {
	return &Configuration{
		PutPriority: 1024,
		PutDelay:    0,
		PutTtr:      time.Second * 10,
	}
}

func NewCli(config *Configuration, handler container.Handler) *Cli {
	cli := &Cli{Configuration: config}

	cli.container = handler

	return cli
}

func (cli *Cli) Init() (err error) {
	//need to check ENV if we test Cli using mock Env
	cli.Url = util.CheckEnvForValue(EnvUrl, cli.Url)
	cli.Tubes = util.CheckEnvForArray(EnvTubes, cli.Tubes)

	err = validateConfig(cli.Configuration)

	if err != nil {
		return err
	}

	config := cli.container.Connection().Config()

	config.Url = cli.Url
	config.Tubes = cli.Tubes

	err = cli.container.Init(cli.ConfigFile)

	if err != nil {
		return err
	}

	return nil
}

func (cli *Cli) Close() error {
	return cli.container.Close()
}

func (cli *Cli) Container() container.Handler {
	return cli.container
}

func (cli *Cli) PutFromFile() (id uint64, err error) {
	taskData, err := ioutil.ReadFile(cli.TaskDataFile)

	if err != nil {
		return 0, err
	}

	return cli.Put(taskData)
}

func (cli *Cli) Put(taskData []byte) (id uint64, err error) {
	//test data before sending to Beanstalkd
	err = json.Unmarshal(taskData, &common.Task{})

	if err != nil {
		return 0, err
	}

	ch := cli.container.ConnectionHandler()

	id, err = ch.Put(taskData, cli.PutPriority, cli.PutDelay, cli.PutTtr)

	if err != nil {
		return 0, err
	}

	return id, nil
}

func (cli *Cli) Delete(id uint64) error {
	ch := cli.container.ConnectionHandler()

	return ch.Delete(id)
}

func (cli *Cli) Start(callback OsSignalCallback) (err error) {
	w := cli.container.Worker()
	c := cli.container.Consumer()

	w.StartWorker()
	err = c.StartConsumer()

	if err != nil {
		return err
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	if callback != nil {
		callback(sigs)
	}

	done := make(chan bool, 1)

	go func() {
		select {
		case <-sigs:
			done <- true
		}
	}()

	<-done

	c.StopConsumer()
	w.StopWorker()

	//TODO: wait for tasks to finish or timeout

	return nil
}

func (cli *Cli) WriteDefaultConfiguration(writer io.Writer) (n int, err error) {
	var bytes []byte

	bytes, err = json.MarshalIndent(cli.container.Config(), "", " ")

	if err != nil {
		return 0, err
	}

	return writer.Write(bytes)
}

func (cli *Cli) WriteDefaultConfigurationToFile(name string) (n int, err error) {
	file, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0644)

	if err != nil {
		return 0, nil
	}

	defer func() {
		err = file.Close()
	}()

	return cli.WriteDefaultConfiguration(file)
}

//var conn, _ = beanstalk.Dial("tcp", "127.0.0.1:11300")
//
//func Example_reserve() {
//	id, body, err := conn.Reserve(5 * time.Second)
//	if err != nil {
//		panic(err)
//	}
//	fmt.Println("job", id)
//	fmt.Println(string(body))
//}
//
//func Example_reserveOtherTubeSet() {
//	tubeSet := beanstalk.NewTubeSet(conn, "mytube1", "mytube2")
//	id, body, err := tubeSet.Reserve(10 * time.Hour)
//	if err != nil {
//		panic(err)
//	}
//	fmt.Println("job", id)
//	fmt.Println(string(body))
//}
//
//func Example_put() {
//	id, err := conn.Put([]byte("myjob"), 1, 0, time.Minute)
//	if err != nil {
//		panic(err)
//	}
//	fmt.Println("job", id)
//}
//
//func Example_putOtherTube() {
//	tube := &beanstalk.Tube{Conn: conn, Name: "mytube"}
//	id, err := tube.Put([]byte("myjob"), 1, 0, time.Minute)
//	if err != nil {
//		panic(err)
//	}
//	fmt.Println("job", id)
//}
