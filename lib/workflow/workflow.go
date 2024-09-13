package workflow

import (
	"context"
	"log/slog"
	"oliverbutler/lib/environment"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

type WorkflowService struct {
	Client *client.Client
	worker *worker.Worker
}

func NewWorkflowService(environment *environment.EnvironmentService) (*WorkflowService, error) {
	c, err := client.Dial(client.Options{
		HostPort: environment.GetTemporalHost(),
	})
	if err != nil {
		slog.Error("Unable to create client", "error", err)
		return nil, err
	}

	w := worker.New(c, "oliverbutler", worker.Options{})

	return &WorkflowService{
		Client: &c,
		worker: &w,
	}, nil
}

func (w *WorkflowService) RegisterWorkflow(name string, workflowFunc interface{}) {
	(*w.worker).RegisterWorkflowWithOptions(workflowFunc, workflow.RegisterOptions{Name: name})
}

func (w *WorkflowService) RegisterActivity(activity interface{}) {
	(*w.worker).RegisterActivity(activity)
}

func (w *WorkflowService) ExecuteWorkflow(context context.Context, options client.StartWorkflowOptions, workflow string, args interface{}) error {
	we, err := (*w.Client).ExecuteWorkflow(context, options, workflow, args)
	if err != nil {
		slog.Error("Unable to execute workflow", "error", err)
	}

	slog.Info("Started workflow", "WorkflowID", we.GetID(), "RunID", we.GetRunID())

	return err
}

func (w *WorkflowService) StartBackgroundWorker() {
	go w.startWorker()
}

func (w *WorkflowService) startWorker() {
	err := (*w.worker).Run(worker.InterruptCh())
	if err != nil {
		slog.Error("Unable to start worker", "error", err)
	}
}

func (w *WorkflowService) TearDown() {
	slog.Info("Tearing down workflow service")
	(*w.Client).Close()
	(*w.worker).Stop()
}
