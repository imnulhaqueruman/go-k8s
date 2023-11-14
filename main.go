package main

import (
    "context"
    "fmt"
    "os"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"

	"k8s.io/client-go/util/retry"
	exec "k8s.io/client-go/util/exec"

)

func main() {
    // Use the current context in kubeconfig
    kubeconfig := os.Getenv("HOME") + "/.kube/config"

    // Build kubeconfig from a file
    config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
    if err != nil {
        panic(err.Error())
    }

    // Create a Kubernetes clientset
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        panic(err.Error())
    }

    // Example: List Pods in the default namespace
    pods, err := clientset.CoreV1().Pods("ubuntu").List(context.TODO(), metav1.ListOptions{})
    if err != nil {
        panic(err.Error())
    }

    fmt.Println("Pods in the default namespace:")
	
    namespace := "ubuntu"
    containerName := "nginx"
    command := []string{"echo", "Hello, Kubernetes!"}
	// fmt.Printf(" - %s\n", pods.Items.Name)
    for _, pod := range pods.Items {
        fmt.Printf(" - %s\n", pod.Name)
		podName := pod.Name
		err = execCommand(clientset, podName, namespace, containerName, command)
    if err != nil {
        panic(err.Error())
    }
    }
}
func execCommand(clientset *kubernetes.Clientset, podName, namespace, containerName string, command []string) error {
    req := clientset.CoreV1().RESTClient().
        Post().
        Resource("pods").
        Name(podName).
        Namespace(namespace).
        SubResource("exec").
        Param("container", containerName).
        Param("stdin", "true").
        Param("stdout", "true").
        Param("stderr", "true").
        Param("tty", "true").
        Param("command", command...)

    exec, err := remotecommand.NewSPDYExecutor(clientset.Config, "POST", req.URL())
    if err != nil {
        return err
    }

    return exec.Stream(remotecommand.StreamOptions{
        IOStreams: remotecommand.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
        Tty:       true,
    })
}
