//go:generate mockgen -destination=./mocks/mock_worker.go -package=mocks . EventHandler
//Package workers provides primitives for configuration and starting worker queues
package worker

import (
	"github.com/mnikita/task-queue/pkg/common"
	"github.com/mnikita/task-queue/pkg/connector"
	"github.com/mnikita/task-queue/pkg/log"
	"github.com/mnikita/task-queue/pkg/util"
	"sync"
	"time"
)

type EventHandler interface {
	OnStartWorker()
	OnEndWorker()

	OnPreTask(task *common.Task)
	OnPostTask(task *common.Task)
	OnThreadHeartbeat(threadId int)
}

//Worker stores configuration for server activation
type Worker struct {
	taskQueue     chan *common.Task
	taskQueueQuit chan bool
	quit          chan bool

	config *Configuration

	taskEventHandler common.TaskProcessEventHandler
	eventHandler     EventHandler

	taskQueueCounter int
	mux              sync.Mutex
}

//Configuration stores initialization data for worker server
type Configuration struct {
	//Number of pre-initialized task thread
	Concurrency int

	//Waiting time to accept new consumer task for processing
	WaitToAcceptConsumerTask time.Duration

	//Waiting time for worker to wait task threads closing
	WaitTaskThreadsToClose time.Duration

	//Waiting time to issue heartbeat
	Heartbeat time.Duration
}

func (w *Worker) handle(wg *sync.WaitGroup) {
	defer wg.Done()

	w.mux.Lock()
	w.taskQueueCounter++
	id := w.taskQueueCounter
	w.mux.Unlock()

	log.Logger().TaskThreadStarted(id)
	defer log.Logger().TaskThreadEnded(id)

	for {
		select {
		case <-w.taskQueueQuit:
			return
		case task := <-w.taskQueue:
			if task != nil {
				w.handleTask(id, task)
			} else {
				return
			}
		case <-time.After(w.config.Heartbeat):
			w.OnThreadHeartbeat(id)
		}
	}
}

func (w *Worker) handleTask(threadId int, task *common.Task) {
	taskHandler, err := common.GetRegisteredTaskHandler(task)

	if err != nil {
		w.OnTaskError(task, common.NewTaskThreadError(task, err))
	} else {
		w.OnPreTask(task, threadId)

		taskHandler.SetTaskProcessEventHandler(w)
		taskHandler.SetTask(task)

		err = taskHandler.Handle()

		if err != nil {
			w.OnTaskError(task, common.NewTaskThreadError(task, err))
		} else {
			w.OnPostTask(task, threadId)

			w.OnTaskSuccess(task)
		}
	}
}

func (w *Worker) startTaskThreads(waitGroup *sync.WaitGroup) {
	for i := 0; i < w.config.Concurrency; i++ {
		waitGroup.Add(1)
		go w.handle(waitGroup)
	}
}

func (w *Worker) stopTaskThreads(waitGroup *sync.WaitGroup) {
	log.Logger().TaskThreadsStopping(w.config.Concurrency)

	for i := 0; i < w.config.Concurrency; i++ {
		w.taskQueueQuit <- true
	}

	err := util.WaitTimeout(waitGroup, w.config.WaitTaskThreadsToClose)

	if err != nil {
		log.Logger().Error(err)
	}
}

func NewConfiguration() *Configuration {
	//make default configuration
	return &Configuration{
		Concurrency:              util.GetSystemConcurrency(),
		Heartbeat:                time.Second * 5,
		WaitToAcceptConsumerTask: time.Second * 30,
		WaitTaskThreadsToClose:   time.Second * 30,
	}
}

//NewWorker creates and configures Worker instance
func NewWorker(config *Configuration, conn *connector.Connector) *Worker {
	if util.IsNil(config) {
		config = NewConfiguration()
	}

	var quit = make(chan bool)
	var taskQueueQuit = make(chan bool, config.Concurrency)
	var taskQueue = make(chan *common.Task, config.Concurrency)

	worker := &Worker{config: config, taskQueue: taskQueue, quit: quit,
		taskQueueQuit: taskQueueQuit}

	conn.SetTaskQueueChannel(taskQueue)

	return worker
}

//SetEventHandler
func (w *Worker) SetEventHandler(eventHandler EventHandler) {
	w.eventHandler = eventHandler
}

//SetTaskEventHandler
func (w *Worker) SetTaskEventHandler(eventHandler common.TaskProcessEventHandler) {
	w.taskEventHandler = eventHandler
}

//StartWorker starts workers server
func (w *Worker) StartWorker() {
	w.OnStartWorker()

	var waitGroup sync.WaitGroup

	w.startTaskThreads(&waitGroup)

	go func(wg *sync.WaitGroup) {
		<-w.quit
		w.stopTaskThreads(wg)

		w.quit <- true

		w.OnEndWorker()

		return
	}(&waitGroup)
}

//StopWorker stops workers server
func (w *Worker) StopWorker() {
	log.Logger().WorkerStopping()

	//send stop signal to worker thread
	w.quit <- true

	close(w.taskQueue)

	//wait for worker thread stop confirmation
	<-w.quit
	close(w.quit)
}

func (w *Worker) OnStartWorker() {
	log.Logger().WorkerStarted()

	if !util.IsNil(w.eventHandler) {
		w.eventHandler.OnStartWorker()
	}
}

func (w *Worker) OnEndWorker() {
	log.Logger().WorkerEnded()

	if !util.IsNil(w.eventHandler) {
		w.eventHandler.OnEndWorker()
	}
}

func (w *Worker) OnPreTask(task *common.Task, threadId int) {
	log.Logger().TaskPre(task.Name, threadId)

	if !util.IsNil(w.eventHandler) {
		w.eventHandler.OnPreTask(task)
	}
}

func (w *Worker) OnPostTask(task *common.Task, threadId int) {
	log.Logger().TaskPost(task.Name, threadId)

	if !util.IsNil(w.eventHandler) {
		w.eventHandler.OnPostTask(task)
	}
}

func (w *Worker) OnThreadHeartbeat(threadId int) {
	log.Logger().ThreadHeartbeat(threadId, w.config.Heartbeat)

	if !util.IsNil(w.eventHandler) {
		w.eventHandler.OnThreadHeartbeat(threadId)
	}
}

func (w *Worker) OnTaskResult(task *common.Task, a ...interface{}) {
	log.Logger().TaskResult(task.Name, a)

	if !util.IsNil(w.taskEventHandler) {
		w.taskEventHandler.OnTaskResult(task, a)
	}
}

func (w *Worker) OnTaskSuccess(task *common.Task) {
	log.Logger().TaskSuccess(task.Name)

	if !util.IsNil(w.taskEventHandler) {
		w.taskEventHandler.OnTaskSuccess(task)
	}
}

func (w *Worker) OnTaskHeartbeat(task *common.Task) {
	log.Logger().TaskHeartbeat(task.Name)

	if !util.IsNil(w.taskEventHandler) {
		w.taskEventHandler.OnTaskHeartbeat(task)
	}
}

func (w *Worker) OnTaskError(task *common.Task, err error) {
	log.Logger().Error(err)

	if !util.IsNil(w.taskEventHandler) {
		w.taskEventHandler.OnTaskError(task, err)
	}
}
