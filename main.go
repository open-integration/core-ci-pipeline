package main

import (
	"fmt"
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
					Version: "0.8.0",
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
							}, []string{}, wfcontext),
						}
					},
				},
				core.EventReaction{
					Condition: core.ConditionTaskFinishedWithStatus("clone", state.TaskStatusSuccess),
					Reaction: func(ev state.Event, state state.State) []task.Task {
						return []task.Task{
							buildKubeRunTask("download-binaries", []string{
								"cd core",
								"go mod download",
							}, []string{
								"GOCACHE=/openintegration/gocache",
								// "GOPATH=/openintegration/gopath",
								"GO111MODULE=on",
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
								// "make test",
							}, []string{
								"GOCACHE=/openintegration/gocache",
								// "GOPATH=/openintegration/gopath",
								"GO111MODULE=on",
							}, wfcontext),
							buildKubeRunTask("test-fmt", []string{
								"cd core",
								"make test-fmt",
							}, []string{}, wfcontext),
							buildKubeRunTask("export-vars", []string{
								"cd core",
								"make export-vars",
							}, []string{}, wfcontext),
						}
					},
				},
				core.EventReaction{
					Condition: core.ConditionTaskFinishedWithStatus("test", state.TaskStatusSuccess),
					Reaction: func(ev state.Event, state state.State) []task.Task {
						if true || os.Getenv("CODECOV_TOKEN") == "" {
							return []task.Task{}
						}
						envs := []string{
							fmt.Sprintf("CODECOV_TOKEN=%s", os.Getenv("CODECOV_TOKEN")),
						}
						if os.Getenv("CI_BUILD_ID") != "" {
							envs = append(envs, fmt.Sprintf("CI_BUILD_ID=%s", os.Getenv("CI_BUILD_ID")))
						}
						if os.Getenv("CI_BUILD_URL") != "" {
							envs = append(envs, fmt.Sprintf("CI_BUILD_URL=%s", os.Getenv("CI_BUILD_URL")))
						}
						return []task.Task{
							buildKubeRunTask("codecov", []string{
								"cd core",
								"curl -o codecov.sh https://codecov.io/bash && chmod +x codecov.sh",
								"export VCS_BRANCH_NAME=$(cat ./branch.var)",
								"export VCS_SLUG=$(cat ./slug.var)",
								"export VCS_COMMIT_ID=$(cat ./commit.var)",
								"./codecov.sh -e VCS_COMMIT_ID,VCS_BRANCH_NAME,VCS_SLUG,CI_BUILD_ID,CI_BUILD_URL",
							}, envs, wfcontext),
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
