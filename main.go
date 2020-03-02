package main

import (
	"os"

	"github.com/open-integration/core"
	"github.com/open-integration/core/pkg/state"
	"github.com/open-integration/core/pkg/task"
)

const (

	// PipelineName name of the pipeline to be used by core.engine
	PipelineName = "core-ci"

	// CreatePVCTask uniqe name for the create-pvc task
	CreatePVCTask = "create-pvc"
)

type (
	workflowcontext struct {
		kube    kubeCredentials
		pvc     string
		logsdir string
	}
	kubeCredentials struct {
		host      string
		crt       string
		token     string
		namespace string
	}
)

func main() {

	kube, err := buildKubeCredentials()
	dieOnError(err)
	wfcontext := &workflowcontext{
		kube: *kube,
	}
	updateWorkflowContextWithPVC(wfcontext)
	updateWorkflowContextWithLogDirectory(wfcontext)

	pipe := core.Pipeline{
		Metadata: core.PipelineMetadata{
			Name: PipelineName,
		},
		Spec: core.PipelineSpec{
			Services: []core.Service{
				core.Service{
					As:      "kubernetes",
					Name:    "kubernetes",
					Version: "0.3.0",
				},
			},
			Reactions: []core.EventReaction{
				core.EventReaction{
					Condition: core.ConditionEngineStarted,
					Reaction:  reactToEngineStartedEvent(wfcontext),
				},
				core.EventReaction{
					Condition: core.ConditionTaskFinished(CreatePVCTask),
					Reaction: func(ev state.Event, state state.State) []task.Task {
						return []task.Task{
							buildKubeRunTask("clone", []string{
								"rm -rf core",
								"git clone https://github.com/open-integration/core",
							}, wfcontext),
						}
					},
				},
				core.EventReaction{
					Condition: core.ConditionTaskFinishedWithStatus("clone", state.TaskStatusSuccess),
					Reaction: func(ev state.Event, state state.State) []task.Task {
						return []task.Task{
							buildKubeRunTask("download-binaries", []string{
								// azurefile mount bug
								"sleep 2",
								"cd core",
								"go mod tidy",
							}, wfcontext),
						}
					},
				},
				core.EventReaction{
					Condition: core.ConditionTaskFinishedWithStatus("download-binaries", state.TaskStatusSuccess),
					Reaction: func(ev state.Event, state state.State) []task.Task {
						return []task.Task{
							buildKubeRunTask("test", []string{
								"cd core",
								"mkdir .cover",
								"make test",
							}, wfcontext),
							buildKubeRunTask("test-fmt", []string{
								"sleep 2",
								"cd core",
								"make test-fmt",
							}, wfcontext),
						}
					},
				},
			},
		},
	}

	opt := &core.EngineOptions{
		Pipeline: pipe,
	}
	if os.Getenv("IN_CLUSTER") != "" {
		opt.Kubeconfig = &core.EngineKubernetesOptions{
			B64Crt:              wfcontext.kube.crt,
			Host:                wfcontext.kube.host,
			Token:               wfcontext.kube.token,
			InCluster:           true,
			LogsVolumeClaimName: wfcontext.pvc,
			LogsVolumeName:      wfcontext.pvc,
			Namespace:           kube.namespace,
		}
	}
	if wfcontext.logsdir != "" {
		opt.LogsDirectory = wfcontext.logsdir
	}
	e := core.NewEngine(opt)
	core.HandleEngineError(e.Run())
}

func reactToEngineStartedEvent(wfcontext *workflowcontext) func(ev state.Event, state state.State) []task.Task {
	return func(ev state.Event, state state.State) []task.Task {
		return []task.Task{
			task.Task{
				Metadata: task.Metadata{
					Name: CreatePVCTask,
				},
				Spec: task.Spec{
					Service:  "kubernetes",
					Endpoint: "createpvc",
					Arguments: []task.Argument{
						buildAuthTaskArgument(wfcontext),
						task.Argument{
							Key:   "PVC",
							Value: buildPvcString(wfcontext.kube.namespace, wfcontext.pvc),
						},
					},
				},
			},
		}
	}
}
