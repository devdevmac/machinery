package machinery

import (
	"fmt"
	"log"

	"github.com/RichardKnop/machinery/v1/backends"
	"github.com/RichardKnop/machinery/v1/brokers"
	"github.com/RichardKnop/machinery/v1/config"
	"github.com/RichardKnop/machinery/v1/signatures"
	"github.com/twinj/uuid"
)

// Server is the main Machinery object and stores all configuration
// All the tasks workers process are registered against the server
type Server struct {
	config          *config.Config
	registeredTasks map[string]interface{}
	broker          brokers.Broker
	backend         backends.Backend
}

// NewServer creates Server instance
func NewServer(cnf *config.Config) (*Server, error) {
	broker, err := BrokerFactory(cnf)
	if err != nil {
		return nil, err
	}

	// Backend is optional so we ignore the error
	backend, _ := BackendFactory(cnf)

	return &Server{
		config:          cnf,
		registeredTasks: make(map[string]interface{}),
		broker:          broker,
		backend:         backend,
	}, nil
}

// NewWorker creates Worker instance
func (server *Server) NewWorker(consumerTag string) *Worker {
	return &Worker{
		server:      server,
		ConsumerTag: consumerTag,
	}
}

// GetBroker returns connection object
func (server *Server) GetBroker() brokers.Broker {
	return server.broker
}

// GetConfig returns connection object
func (server *Server) GetConfig() *config.Config {
	return server.config
}

// RegisterTasks registers all tasks at once
func (server *Server) RegisterTasks(tasks map[string]interface{}) {
	server.registeredTasks = tasks
}

// RegisterTask registers a single task
func (server *Server) RegisterTask(name string, task interface{}) {
	server.registeredTasks[name] = task
}

// GetRegisteredTask returns registered task by name
func (server *Server) GetRegisteredTask(name string) interface{} {
	return server.registeredTasks[name]
}

// SendTask publishes a task to the default queue
func (server *Server) SendTask(signature *signatures.TaskSignature) (*backends.AsyncResult, error) {
	// Auto generate a UUID if not set already
	if signature.UUID == "" {
		signature.UUID = uuid.NewV4().String()
	}

	if err := server.broker.Publish(signature); err != nil {
		return nil, fmt.Errorf("Publish Message: %v", err)
	}

	// Update task state to PENDING
	taskState := backends.NewPendingTaskState(signature.UUID)
	if err := server.UpdateTaskState(taskState); err != nil {
		log.Print(err)
	}

	return backends.NewAsyncResult(signature.UUID, server.backend), nil
}

// SendChain triggers a chain of tasks
func (server *Server) SendChain(chain *Chain) (*backends.ChainAsyncResult, error) {
	if err := server.broker.Publish(chain.Tasks[0]); err != nil {
		return nil, fmt.Errorf("Publish Message: %v", err)
	}

	// Update task state to PENDING
	taskState := backends.NewPendingTaskState(chain.Tasks[0].UUID)
	if err := server.UpdateTaskState(taskState); err != nil {
		log.Print(err)
	}

	return backends.NewChainAsyncResult(chain.GetUUIDs(), server.backend), nil
}

// UpdateTaskState updates a task state
// If no result backend has been configured, does nothing
func (server *Server) UpdateTaskState(taskState *backends.TaskState) error {
	if server.backend == nil {
		return nil
	}
	return server.backend.UpdateState(taskState)
}
