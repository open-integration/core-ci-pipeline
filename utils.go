package main

import (
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"

	"github.com/open-integration/core/pkg/task"
	"github.com/open-integration/service-catalog/kubernetes/pkg/endpoints/run"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type (
	kubeRunTaskOptions struct {
		name      string
		commands  []string
		environ   []string
		wfcontext *workflowcontext
		image     string
	}
)

func buildCommand(cmds []string) string {
	command := []string{
		"set -e",
	}
	command = append(command, cmds...)
	return strings.Join(command, " ; ")
}

func buildPodString(namespace string, name string, cmd string, environ []string, image string, pvc string) string {
	envs := []v1.EnvVar{}
	for _, e := range environ {
		kv := strings.Split(e, "=")
		envs = append(envs, v1.EnvVar{
			Name:  kv[0],
			Value: kv[1],
		})
	}
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.ReplaceAll(name, " ", "-"),
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
					Env: envs,
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

func buildPvcString(namespace string, pvc string) string {
	sc := "azurefile"
	p, _ := json.Marshal(&v1.PersistentVolumeClaim{
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
	})
	return string(p)
}

func buildKubeRunTask(opt *kubeRunTaskOptions) task.Task {
	image := "openintegration/testing"
	if opt.image != "" {
		image = opt.image
	}
	return task.Task{
		Metadata: task.Metadata{
			Name: opt.name,
		},
		Spec: task.Spec{
			Service:  "kubernetes",
			Endpoint: "run",
			Arguments: []task.Argument{
				buildAuthTaskArgument(opt.wfcontext),
				task.Argument{
					Key:   "Timeout",
					Value: 120,
				},
				task.Argument{
					Key:   "Pod",
					Value: buildPodString(opt.wfcontext.kube.namespace, opt.name, buildCommand(opt.commands), opt.environ, image, opt.wfcontext.pvc),
				},
			},
		},
	}
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

func getEnvOrDie(name string) string {
	v := os.Getenv(name)
	if v == "" {
		dieOnError(fmt.Errorf("Variable \"%s\" is not set, exiting", name))
	}
	return v
}

func dieOnError(err error) {
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func updateWorkflowContextWithPVC(wfctx *workflowcontext) {
	if p := os.Getenv("PVC_NAME"); p != "" {
		wfctx.pvc = p
		return
	}
	wfctx.pvc = "core-ci-pvc"
}

func updateWorkflowContextWithLogDirectory(wfctx *workflowcontext) {
	if p := os.Getenv("LOG_DIRECTORY"); p != "" {
		wfctx.logsdir = p
		return
	}
	wfctx.logsdir = ""
}
