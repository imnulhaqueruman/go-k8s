package main

import (
	"context"
    "fmt"
	"log"
	"io"
	"os"
	"net/http"
	"os/exec"
	"sync"

	"github.com/gorilla/websocket"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	// "golang.org/x/crypto/ssh/terminal"
)


var ptyProcess *exec.Cmd
var ptyInPipe io.WriteCloser
var ptyOutPipe io.ReadCloser
var ptyMutex sync.Mutex


func startPtyProcess(namespace, podName string, clientset *kubernetes.Clientset, config *rest.Config) {
	ptyMutex.Lock()
	defer ptyMutex.Unlock()

	// Use the Kubernetes API to execute a command in the specified pod
	// Note: This is just an example, adjust it based on your requirements
	cmd := []string{"sh", "-c", "echo Hello from pod " + podName}
	req := clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		Param("container", "nginx").
		Param("stdin", "true").
		Param("stdout", "true").
		Param("stderr", "true").
		Param("tty", "true").
		Param("command", cmd)

	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		log.Fatal(err)
	}

	err = executor.Stream(remotecommand.StreamOptions{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ptyProcess = exec.Command()

	var err error
	ptyInPipe, err = ptyProcess.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}

	ptyOutPipe, err = ptyProcess.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	err = ptyProcess.Start()
	if err != nil {
		log.Fatal(err)
	}
}

func handleConnection(ws *websocket.Conn) {
	fmt.Println("New session")
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
	// fmt.Printf(" - %s\n", pods.Items.Name)
    for _, pod := range pods.Items {
        fmt.Printf(" - %s\n", pod.Name)
		startPtyProcess("ubuntu",pod.Name)
    }
	// Start the pty process
	 // You can change the shell as needed

	go func() {
		defer ws.Close()
		defer ptyInPipe.Close()

		for {
			// Read from WebSocket and write to pty
			_, msg, err := ws.ReadMessage()
			if err != nil {
				log.Println(err)
				break
			}
			ptyInPipe.Write(msg)
		}
	}()

	go func() {
		defer ws.Close()

		// Read from pty and write to WebSocket
		buf := make([]byte, 1024)
		for {
			n, err := ptyOutPipe.Read(buf)
			if err != nil {
				log.Println(err)
				break
			}
			ws.WriteMessage(websocket.TextMessage, buf[:n])
		}
	}()
}

func main() {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		handleConnection(conn)
	})

	fmt.Println("WebSocket server is up and running on :6060")
	log.Fatal(http.ListenAndServe(":6060", nil))
}

