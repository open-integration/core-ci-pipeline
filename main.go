package main

import (
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"

	"github.com/open-integration/core"
	"github.com/open-integration/core/pkg/state"
	"github.com/open-integration/core/pkg/task"
	"github.com/open-integration/service-catalog/kubernetes/pkg/endpoints/run"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// CreatePVCTask uniqe name for the create-pvc task
	CreatePVCTask = "create-pvc"
)

type (
	workflowcontext struct {
		kube kubeCredentials
	}
	kubeCredentials struct {
		host      string
		crt       string
		token     string
		namespace string
	}
)

var (
	pvc     = "core-ci-pvc"
	logsdir = ""
)

func main() {
	if p := os.Getenv("PVC_NAME"); p != "" {
		pvc = p
	}

	if l := os.Getenv("LOG_DIRECTORY"); l != "" {
		logsdir = l
	}

	kube, err := buildKubeCredentials()
	dieOnError(err)
	wfcontext := &workflowcontext{
		kube: *kube,
	}

	pipe := core.Pipeline{
		Metadata: core.PipelineMetadata{
			Name: "core-ci",
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
					Condition: func(ev state.Event, s state.State) bool {
						if ev.Metadata.Name != state.EventTaskFinished {
							return false
						}
						for _, t := range s.Tasks() {
							if t.State == state.TaskStateFinished && t.Task.Metadata.Name == CreatePVCTask {
								return true
							}
						}
						return false
					},
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
								"sleep 10",
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
			LogsVolumeClaimName: pvc,
			LogsVolumeName:      pvc,
			Namespace:           kube.namespace,
		}
	}
	if logsdir != "" {
		opt.LogsDirectory = logsdir
	}
	e := core.NewEngine(opt)
	core.HandleEngineError(e.Run())
}

func buildPodString(namespace string, name string, cmd string, image string) string {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				v1.Container{
					Name:  "container-1",
					Image: image,
					Command: []string{
						"sh",
						"-c",
					},
					Args: []string{
						cmd,
					},
					WorkingDir: "/openintegration",
					VolumeMounts: []v1.VolumeMount{
						v1.VolumeMount{
							MountPath: "/openintegration",
							Name:      pvc,
						},
					},
				},
			},
			Volumes: []v1.Volume{
				v1.Volume{
					Name: pvc,
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc,
						},
					},
				},
			},
		},
	}
	p, _ := json.Marshal(pod)
	return string(p)
}

func buildPvcString(namespace string) string {
	sc := "azurefile"
	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvc,
			Namespace: namespace,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadWriteMany,
			},
			StorageClassName: &sc,
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					"storage": resource.MustParse("1Gi"),
				},
			},
		},
	}
	p, _ := json.Marshal(pvc)
	return string(p)
}

func buildCommand(cmds []string) string {
	command := []string{
		"set -e",
	}
	command = append(command, cmds...)
	return strings.Join(command, " ; ")
}

func buildAuthTaskArgument(wfcontext *workflowcontext) task.Argument {
	return task.Argument{
		Key: "Auth",
		Value: &run.Auth{
			Type:  run.KubernetesServiceAccount,
			CRT:   &wfcontext.kube.crt,
			Host:  &wfcontext.kube.host,
			Token: &wfcontext.kube.token,
		},
	}
}

func getEnvOrDie(name string) string {
	v := os.Getenv(name)
	if v == "" {
		dieOnError(fmt.Errorf("Variable \"%s\" is not set, exiting", name))
	}
	return v
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
							Value: buildPvcString(wfcontext.kube.namespace),
						},
					},
				},
			},
		}
	}
}

func dieOnError(err error) {
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func buildKubeRunTask(name string, commands []string, wfcontext *workflowcontext) task.Task {
	return task.Task{
		Metadata: task.Metadata{
			Name: name,
		},
		Spec: task.Spec{
			Service:  "kubernetes",
			Endpoint: "run",
			Arguments: []task.Argument{
				buildAuthTaskArgument(wfcontext),
				task.Argument{
					Key:   "Timeout",
					Value: 120,
				},
				task.Argument{
					Key:   "Pod",
					Value: buildPodString(wfcontext.kube.namespace, name, buildCommand(commands), "openintegration/testing"),
				},
			},
		},
	}
}

func buildKubeCredentials() (*kubeCredentials, error) {
	if os.Getenv("IN_CLUSTER") == "true" {
		const (
			tokenFile     = "/var/run/secrets/kubernetes.io/serviceaccount/token"
			rootCAFile    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
			namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
		)
		host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
		if len(host) == 0 || len(port) == 0 {
			return nil, fmt.Errorf("ErrNotInCluster")
		}

		token, err := ioutil.ReadFile(tokenFile)
		if err != nil {
			return nil, err
		}

		namespace, err := ioutil.ReadFile(namespaceFile)
		if err != nil {
			return nil, err
		}

		crt, err := ioutil.ReadFile(rootCAFile)
		if err != nil {
			return nil, err
		}
		return &kubeCredentials{
			crt:       b64.StdEncoding.EncodeToString(crt),
			token:     string(token),
			host:      "https://" + net.JoinHostPort(host, port),
			namespace: string(namespace),
		}, nil
	}
	return &kubeCredentials{
		crt:       getEnvOrDie("KUBERNETES_B64_CRT"),
		token:     getEnvOrDie("KUBERNETES_TOKEN"),
		host:      getEnvOrDie("KUBERNETES_HOST"),
		namespace: getEnvOrDie("KUBERNETES_NAMESPACE"),
	}, nil
}
