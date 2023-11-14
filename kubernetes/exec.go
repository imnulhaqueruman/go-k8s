package main

import (
    "io"
	"bytes"
	"flag"
	"fmt"
	
	// "os"

	// "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
    v1 "k8s.io/api/core/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/kubernetes/scheme"
    restclient "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/remotecommand"
)

// ExecCmd exec command on specific pod and wait the command's output.
func ExecCmdExample(client kubernetes.Interface, config *restclient.Config, podName string,
    command string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
    cmd := []string{
        "sh",
        "-c",
        command,
    }
    req := client.CoreV1().RESTClient().Post().Resource("pods").Name(podName).
        Namespace("default").SubResource("exec")
    option := &v1.PodExecOptions{
        Command: cmd,
        Stdin:   true,
        Stdout:  true,
        Stderr:  true,
        TTY:     true,
    }
    if stdin == nil {
        option.Stdin = false
    }
    req.VersionedParams(
        option,
        scheme.ParameterCodec,
    )
    exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
    if err != nil {
        return err
    }
    err = exec.Stream(remotecommand.StreamOptions{
        Stdin:  stdin,
        Stdout: stdout,
        Stderr: stderr,
    })
    if err != nil {
        return err
    }

    return nil
}
func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", home+"/.kube/config", "absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	// fmt.Printf("%+v\n", config)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	// fmt.Printf("%+v\n", clientset)
	if err != nil {
		panic(err.Error())
	}

	podName := "shell-demo"
	command := "ls "

	var stdin io.Reader 
	var stdout, stderr bytes.Buffer

	err = ExecCmdExample(clientset, config, podName, command, stdin, &stdout, &stderr)
	if err != nil {
		fmt.Printf("Error executing command: %v\n", err)
		return
	}

	fmt.Printf("Stdout: %s\n", stdout.String())
	fmt.Printf("Stderr: %s\n", stderr.String())
}
