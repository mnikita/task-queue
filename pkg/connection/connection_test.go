package connection_test

import (
	"errors"
	"github.com/golang/mock/gomock"
	"github.com/mnikita/task-queue/pkg/connection"
	"github.com/mnikita/task-queue/pkg/connection/mocks"
	"github.com/mnikita/task-queue/pkg/consumer"
	cmocks "github.com/mnikita/task-queue/pkg/consumer/mocks"
	"github.com/mnikita/task-queue/pkg/util"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

type Mock struct {
	t *testing.T

	bc *connection.Configuration

	ctrl *gomock.Controller

	handler connection.Handler

	dialer *mocks.MockDialer
	conn   *cmocks.MockConnectionHandler
}

func newMock(t *testing.T) *Mock {
	m := &Mock{}
	m.t = t
	m.ctrl = gomock.NewController(t)

	m.dialer = mocks.NewMockDialer(m.ctrl)
	m.conn = cmocks.NewMockConnectionHandler(m.ctrl)

	m.bc = connection.NewConfiguration()

	m.handler = connection.NewConnection(m.bc, m.dialer)

	return m
}

func setupTest(m *Mock) func() {
	if m == nil {
		panic("Mock not initialized")
	}

	// Test teardown - return a closure for use by 'defer'
	return func() {
		defer m.ctrl.Finish()
		defer util.AssertPanic(m.t)
	}
}

func TestInitConnection(t *testing.T) {
	m := newMock(t)

	m.bc.Url = "tcp://127.0.0.1:11300"

	m.dialer.EXPECT().Dial(gomock.Eq("127.0.0.1:11300"), gomock.Nil()).Return(m.conn, nil)
	m.conn.EXPECT().ListTubes().Return([]string{"default"}, nil)
	m.conn.EXPECT().Close()

	defer setupTest(m)()

	assert.Nil(t, m.handler.Init())

	assert.Nil(t, m.conn.Close())
}

func TestBadUrl(t *testing.T) {
	m := newMock(t)

	m.bc.Url = "http"

	m.dialer.EXPECT().Dial(gomock.Eq(""), gomock.Nil()).Return(nil, errors.New("bad url"))

	defer setupTest(m)()

	assert.NotNil(t, m.handler.Init())
}

func TestEmptyUrl(t *testing.T) {
	m := newMock(t)

	m.bc.Url = ""

	m.dialer.EXPECT().Dial(gomock.Eq(""), gomock.Nil()).Return(nil, errors.New("bad url"))

	defer setupTest(m)()

	assert.NotNil(t, m.handler.Init())
}

func TestTubeSet(t *testing.T) {
	m := newMock(t)

	m.bc.Url = "tcp://127.0.0.1:11300"
	m.bc.Tubes = []string{"mika", "pera", "laza"}

	ch := mocks.NewMockChannels(m.ctrl)

	m.dialer.EXPECT().Dial(gomock.Eq("127.0.0.1:11300"), gomock.Eq(m.bc.Tubes)).Return(m.conn, nil)
	m.dialer.EXPECT().CreateChannels().Return(ch)
	m.conn.EXPECT().ListTubes().Return([]string{"mika", "pera", "laza"}, nil)
	m.conn.EXPECT().Close()

	defer setupTest(m)()

	assert.Nil(t, m.handler.Init())

	assert.Nil(t, m.conn.Close())
}

func TestTube(t *testing.T) {
	m := newMock(t)

	m.bc.Url = "tcp://127.0.0.1:11300"
	m.bc.Tubes = []string{"mika"}

	ch := mocks.NewMockChannel(m.ctrl)

	m.dialer.EXPECT().Dial(gomock.Eq("127.0.0.1:11300"), gomock.Eq(m.bc.Tubes)).Return(m.conn, nil)
	m.dialer.EXPECT().CreateChannel().Return(ch)
	m.conn.EXPECT().ListTubes().Return([]string{"mika"}, nil)
	m.conn.EXPECT().Close()

	defer setupTest(m)()

	assert.Nil(t, m.handler.Init())

	assert.Nil(t, m.conn.Close())
}

func TestReserve(t *testing.T) {
	m := newMock(t)

	m.bc.Url = "tcp://127.0.0.1:11300"
	m.bc.Tubes = nil

	m.dialer.EXPECT().Dial(gomock.Eq("127.0.0.1:11300"), gomock.Eq(m.bc.Tubes)).Return(m.conn, nil)
	m.conn.EXPECT().ListTubes().Return([]string{"mika"}, nil)
	m.conn.EXPECT().Reserve(time.Second)

	m.conn.EXPECT().Close()

	defer setupTest(m)()

	assert.Nil(t, m.handler.Init())

	c := m.handler.(consumer.ConnectionHandler)

	_, _, err := c.Reserve(time.Second)
	assert.Nil(t, err)

	assert.Nil(t, m.conn.Close())
}

func TestReserveTubeSeb(t *testing.T) {
	m := newMock(t)

	m.bc.Url = "tcp://127.0.0.1:11300"
	m.bc.Tubes = []string{"mika", "pera", "laza"}

	ch := mocks.NewMockChannels(m.ctrl)
	ch.EXPECT().Reserve(time.Second)

	m.dialer.EXPECT().Dial(gomock.Eq("127.0.0.1:11300"), gomock.Eq(m.bc.Tubes)).Return(m.conn, nil)
	m.dialer.EXPECT().CreateChannels().Return(ch)
	m.conn.EXPECT().ListTubes().Return([]string{"mika"}, nil)

	m.conn.EXPECT().Close()

	defer setupTest(m)()

	assert.Nil(t, m.handler.Init())

	c := m.handler.(consumer.ConnectionHandler)

	_, _, err := c.Reserve(time.Second)
	assert.Nil(t, err)

	assert.Nil(t, m.conn.Close())
}

func TestPutTube(t *testing.T) {
	m := newMock(t)

	m.bc.Url = "tcp://127.0.0.1:11300"
	m.bc.Tubes = []string{"default"}

	ch := mocks.NewMockChannel(m.ctrl)
	ch.EXPECT().Name().Return("default")
	ch.EXPECT().Put([]byte{}, uint32(1), time.Second, time.Second)

	m.dialer.EXPECT().Dial(gomock.Eq("127.0.0.1:11300"), gomock.Eq(m.bc.Tubes)).Return(m.conn, nil)
	m.dialer.EXPECT().CreateChannel().Return(ch)
	m.conn.EXPECT().ListTubes().Return([]string{"default"}, nil)

	m.conn.EXPECT().Close()

	defer setupTest(m)()

	assert.Nil(t, m.handler.Init())

	c := m.handler.(consumer.ConnectionHandler)

	_, err := c.Put([]byte{}, uint32(1), time.Second, time.Second)

	assert.Nil(t, err)

	assert.Nil(t, m.conn.Close())
}

func TestPutFaultTube(t *testing.T) {
	m := newMock(t)

	m.bc.Url = "tcp://127.0.0.1:11300"
	m.bc.Tubes = []string{"mika", "pera", "laza"}

	ch := mocks.NewMockChannels(m.ctrl)

	m.dialer.EXPECT().Dial(gomock.Eq("127.0.0.1:11300"), gomock.Eq(m.bc.Tubes)).Return(m.conn, nil)
	m.dialer.EXPECT().CreateChannels().Return(ch)
	m.conn.EXPECT().ListTubes().Return([]string{"mika"}, nil)

	m.conn.EXPECT().Close()

	defer setupTest(m)()

	assert.Nil(t, m.handler.Init())

	c := m.handler.(consumer.ConnectionHandler)

	_, err := c.Put([]byte{}, uint32(1), time.Second, time.Second)

	//expecting to throw Channel not specified error
	assert.NotNil(t, err)

	assert.Nil(t, m.conn.Close())
}

func TestPutFaultTube2(t *testing.T) {
	m := newMock(t)

	m.bc.Url = "tcp://127.0.0.1:11300"
	m.bc.Tubes = []string{}

	m.dialer.EXPECT().Dial(gomock.Eq("127.0.0.1:11300"), gomock.Eq(m.bc.Tubes)).Return(m.conn, nil)
	m.conn.EXPECT().ListTubes().Return([]string{"default"}, nil)

	m.conn.EXPECT().Close()

	defer setupTest(m)()

	assert.Nil(t, m.handler.Init())

	c := m.handler.(consumer.ConnectionHandler)

	_, err := c.Put([]byte{}, uint32(1), time.Second, time.Second)

	//expecting to throw Channel not specified error
	assert.NotNil(t, err)

	assert.Nil(t, m.conn.Close())
}
