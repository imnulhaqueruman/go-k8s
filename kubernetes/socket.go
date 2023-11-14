package main

import (
	"fmt"
	"log"
	"net/http"
	// "os"

	"github.com/gorilla/websocket"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	// "k8s.io/client-go/util/wait"
	// "k8s.io/client-go/util/workqueue"
	// "k8s.io/client-go/util/clock"
	// "k8s.io/client-go/util/certificate"

	restclient "k8s.io/client-go/rest"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type CommandMessage struct {
	PodName   string
	Namespace string
	Command   string
}
type websocketWrapper struct {
	conn *websocket.Conn
}

func (w *websocketWrapper) Read(p []byte) (int, error) {
	_, data, err := w.conn.ReadMessage()
	if err != nil {
		return 0, err
	}
	copy(p, data)
	return len(data), nil
}

func (w *websocketWrapper) Write(p []byte) (int, error) {
	err := w.conn.WriteMessage(websocket.TextMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	var msg CommandMessage
	err = conn.ReadJSON(&msg)
	if err != nil {
		log.Println(err)
		return
	}

	kubeconfig, err := getKubeconfig()
	if err != nil {
		log.Println(err)
		return
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Println(err)
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Println(err)
		return
	}

	go execCommand(clientset, config, conn, msg)
}

func execCommand(client kubernetes.Interface, config *restclient.Config, conn *websocket.Conn, msg CommandMessage) {
	cmd := []string{
		"sh",
		"-c",
		msg.Command,
	}

	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(msg.PodName).
		Namespace(msg.Namespace).
		SubResource("exec")

	option := &v1.PodExecOptions{
		Command: cmd,
		Stdin:   true,
		Stdout:  true,
		Stderr:  true,
		TTY:     true,
	}

	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		log.Println(err)
		return
	}

	wrapper := &websocketWrapper{conn: conn}
	streamOptions := remotecommand.StreamOptions{
		Stdin:  wrapper,
		Stdout: wrapper,
		Stderr: wrapper,
		Tty:    true,
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := exec.Stream(streamOptions)
		if err != nil {
			log.Println(err)
		}
	}()
}

func getKubeconfig() (string, error) {
	home := homedir.HomeDir()
	if home != "" {
		return home + "/.kube/config", nil
	}
	return "", fmt.Errorf("unable to determine home directory")
}

func main() {
	http.HandleFunc("/ws", handleWS)
	port := 4000
	log.Printf("WebSocket server listening on :%d...\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
