package common_test

import (
	"github.com/mnikita/task-queue/pkg/common"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

type Mock struct {
	t *testing.T
}

func newMock(t *testing.T) (m *Mock) {
	m = &Mock{}
	m.t = t

	return
}

func setupTest(m *Mock) func() {
	if m == nil {
		panic("Mock not initialized")
	}

	return func() {
	}
}

func HandleShortTest(_ interface{}, _ *common.Task, _ common.TaskProcessEventHandler) error {
	return nil
}

func TestMain(m *testing.M) {
	//	log.Logger().Level = logrus.TraceLevel

	common.RegisterTask("short", func() common.TaskHandler {
		return common.NewBaseTaskHandler(HandleShortTest)
	})

	os.Exit(m.Run())
}

func TestGetRegisteredTaskHandler(t *testing.T) {
	defer setupTest(newMock(t))()

	taskHandler, err := common.GetRegisteredTaskHandler(&common.Task{Name: "short", Payload: []byte("{}")})

	assert.Nil(t, err)
	assert.NotNil(t, taskHandler)
}

func TestGetRegisterTasks(t *testing.T) {
	defer setupTest(newMock(t))()

	tasks := common.GetRegisteredTasks()

	assert.Equal(t, []string{"short"}, tasks)
}
