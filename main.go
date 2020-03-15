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
					Condition: core.ConditionEngineStarted(),
					Reaction:  reactToEngineStartedEvent(wfcontext),
				},
				core.EventReaction{
					Condition: core.ConditionTaskFinished(CreatePVCTask),
					Reaction: func(ev state.Event, state state.State) []task.Task {
						return []task.Task{
							buildKubeRunTask(&kubeRunTaskOptions{
								name: "clone",
								commands: []string{
									"rm -rf core",
									"git clone https://github.com/open-integration/core",
								},
								wfcontext: wfcontext,
							}),
						}
					},
				},
				core.EventReaction{
					Condition: core.ConditionTaskFinishedWithStatus("clone", state.TaskStatusSuccess),
					Reaction: func(ev state.Event, state state.State) []task.Task {
						return []task.Task{
							buildKubeRunTask(
								&kubeRunTaskOptions{
									name: "download-binaries",
									commands: []string{
										"cd core",
										"go mod tidy",
									},
									environ: []string{
										"GOCACHE=/openintegration/gocache",
										"GO111MODULE=on",
									},
									wfcontext: wfcontext,
								}),
						}
					},
				},
				core.EventReaction{
					Condition: core.ConditionTaskFinishedWithStatus("download-binaries", state.TaskStatusSuccess),
					Reaction: func(ev state.Event, state state.State) []task.Task {
						return []task.Task{
							buildKubeRunTask(
								&kubeRunTaskOptions{
									name: "test",
									commands: []string{
										"cd core",
										"mkdir .cover",
										"make test",
									},
									environ: []string{
										"GOCACHE=/openintegration/gocache",
										"GO111MODULE=on",
									},
									wfcontext: wfcontext,
								}),
							buildKubeRunTask(
								&kubeRunTaskOptions{
									name: "test-fmt",
									commands: []string{
										"cd core",
										"make test-fmt",
									},
									wfcontext: wfcontext,
								}),
							buildKubeRunTask(&kubeRunTaskOptions{
								name: "export-vars",
								commands: []string{
									"cd core",
									"make export-vars",
								},
								wfcontext: wfcontext,
							}),
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
							buildKubeRunTask(
								&kubeRunTaskOptions{
									name: "codecov",
									commands: []string{
										"cd core",
										"curl -o codecov.sh https://codecov.io/bash && chmod +x codecov.sh",
										"export VCS_BRANCH_NAME=$(cat ./branch.var)",
										"export VCS_SLUG=$(cat ./slug.var)",
										"export VCS_COMMIT_ID=$(cat ./commit.var)",
										"./codecov.sh -e VCS_COMMIT_ID,VCS_BRANCH_NAME,VCS_SLUG,CI_BUILD_ID,CI_BUILD_URL",
									},
									environ:   envs,
									wfcontext: wfcontext,
								}),
						}
					},
				},
				core.EventReaction{
					Condition: core.ConditionTaskFinishedWithStatus("export-vars", state.TaskStatusSuccess),
					Reaction: func(ev state.Event, state state.State) []task.Task {
						if os.Getenv("SNYK_TOKEN") == "" {
							return []task.Task{}
						}
						envs := []string{
							fmt.Sprintf("SNYK_TOKEN=%s", os.Getenv("SNYK_TOKEN")),
						}
						return []task.Task{
							buildKubeRunTask(&kubeRunTaskOptions{
								name: "security-test",
								commands: []string{
									"cd core",
									"snyk auth $SNYK_TOKEN",
									"snyk monitor",
								},
								environ:   envs,
								wfcontext: wfcontext,
							}),
						}
					},
				},
				core.EventReaction{
					Condition: core.ConditionCombined(
						core.ConditionTaskFinishedWithStatus("security-test", state.TaskStatusSuccess),
						core.ConditionTaskFinishedWithStatus("test-fmt", state.TaskStatusSuccess),
						core.ConditionTaskFinishedWithStatus("test", state.TaskStatusSuccess),
						core.ConditionTaskFinishedWithStatus("codecov", state.TaskStatusSuccess),
					),
					Reaction: func(ev state.Event, state state.State) []task.Task {
						envs := []string{}
						if os.Getenv("GITHUB_TOKEN") != "" {
							envs = append(envs, os.Getenv("GITHUB_TOKEN"))
						}
						return []task.Task{
							buildKubeRunTask(&kubeRunTaskOptions{
								name: "new-release",
								commands: []string{
									"cd code",
									"export VERSION=$(cat ./version.var)",
									"git remote rm origin",
									"git remote add origin https://olegsu:$GITHUB_TOKEN@github.com/open-integration/core.git",
									"git tag v$VERSION",
									"git push --tags",
								},
								environ:   envs,
								image:     "codefresh/cli",
								wfcontext: wfcontext,
							}),
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
