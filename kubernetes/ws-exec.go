package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
	// "os"
	"flag"

	"github.com/gorilla/websocket"
	"github.com/gorilla/mux"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	v1 "k8s.io/api/core/v1"
	// "k8s.io/client-go/util/wait"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/client-go/kubernetes/scheme"
     rest "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/remotecommand"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WSHandler struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
	queue     workqueue.RateLimitingInterface
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	params := mux.Vars(r)
	podName := params["podName"]
	namespace := params["namespace"]

	go h.handleWS(conn, namespace, podName)
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
func (h *WSHandler) handleWS(conn *websocket.Conn, namespace, podName string) {
	defer conn.Close()
	cmd := "ls"
	option := &v1.PodExecOptions{
		Command: []string{ "sh",
        "-c",cmd},
		Stdin:   true,
		Stdout:  true,
		Stderr:  true,
		TTY:     true,
	}

	req := h.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)
	// config, err := h.clientset.RestClientConfig()
	// if err != nil {
	// 	log.Println(err)
	// 	return
	// }

	exec, err := remotecommand.NewSPDYExecutor(h.config, "POST", req.URL())
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

	select {
	case <-done:
		log.Println("WebSocket connection closed")
	case <-time.After(300 * time.Second): // Adjust the timeout as needed
		log.Println("WebSocket connection timeout")
	}
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
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	handler := &WSHandler{
		clientset: clientset,
		config:    config,
		queue:     workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	router := mux.NewRouter()
	router.Handle("/ws/{namespace}/{podName}", handler)

	serverAddr := "127.0.0.1:8080"
	fmt.Printf("WebSocket server listening on http://%s\n", serverAddr)
	log.Fatal(http.ListenAndServe(serverAddr, router))
}
