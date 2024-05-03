package main

import (
	"context"
	"fmt"
	"github.com/emicklei/go-restful/v3"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
)

type Streamer struct {
	dialer         func() (net.Conn, error)
	streamToServer func(clientConn *websocket.Conn, serverConn net.Conn, result chan<- error)
	streamToClient func(clientConn *websocket.Conn, serverConn net.Conn, result chan<- error)
}

const (
	localhostIP = "0.0.0.0"
	serverPort  = 8080
	vncEndpoint = "/vnc"
	baseFolder  = "/var/run/kubevirt-private"
	apiVersion  = "/v1"
)

func main() {
	log.Print("KubeVirt VNC sidecar starting...")
	_ = createRestfulWebService()
	server := createHTTPServer(localhostIP, serverPort)

	errors := make(chan error)
	go func() {
		errors <- server.ListenAndServe()
	}()
	// wait for server to exit
	_ = <-errors
}

func createHTTPServer(ip string, port int) *http.Server {
	return &http.Server{
		Addr: fmt.Sprintf("%s:%d", ip, port),
	}
}

func createRestfulWebService() *restful.WebService {
	ws := new(restful.WebService)
	ws.Doc(fmt.Sprintf("KubeVirt VNC proxy \"%s\" API.", apiVersion))
	ws.Path(apiVersion)
	ws.Route(ws.GET(vncEndpoint).
		To(vncRequestHandler).
		Operation(apiVersion + "VNC").
		Doc("Open a websocket connection to connect to VNC."))

	restful.Add(ws)
	return ws
}

type ResponseContent struct {
	Message string `json:"message"`
}

func vncRequestHandler(request *restful.Request, response *restful.Response) {
	log.Print("Inbound VNC Request: ", request.Request.URL.Path)

	//streamer := newRawStreamer()
	//err := streamer.Handle(request, response)
	//if err != nil {
	//	log.Fatal("Failed to handle request: ", err)
	//	return
	//}
	var content ResponseContent
	content.Message = "123"
	_ = response.WriteJson(content, "application/json")
}

func (s *Streamer) Handle(request *restful.Request, response *restful.Response) error {
	serverConn, statusErr := s.dialer()
	if statusErr != nil {
		return statusErr
	}

	upgrader := &websocket.Upgrader{
		ReadBufferSize:  10240,
		WriteBufferSize: 10240,
		CheckOrigin: func(_ *http.Request) bool {
			return true
		},
		Subprotocols: []string{"plain.kubevirt.io"},
	}
	clientConn, err := upgrader.Upgrade(response, request.Request, nil)

	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(request.Request.Context())
	defer cancel()
	go func(ctx context.Context, clientConn *websocket.Conn, serverConn net.Conn) {
		<-ctx.Done()
		_ = serverConn.Close()
		_ = clientConn.Close()
	}(ctx, clientConn, serverConn)

	results := make(chan error, 2)
	defer close(results)

	go s.streamToClient(clientConn, serverConn, results)
	go s.streamToServer(clientConn, serverConn, results)

	result1 := <-results
	// start canceling on the first result to force all goroutines to terminate
	cancel()
	result2 := <-results

	if result1 != nil {
		return result1
	}
	return result2
}

func newRawStreamer() *Streamer {
	return &Streamer{
		dialer: func() (net.Conn, error) {
			uuid, err := resolveUUIDFolder()
			if err != nil {
				return nil, fmt.Errorf("error resolving UUID folder: %v", err)
			}
			socketPath := fmt.Sprintf("%s/%s/virt-vnc", baseFolder, uuid)
			return net.Dial("unix", socketPath)
		},
		streamToServer: func(clientConn *websocket.Conn,
			serverConn net.Conn, result chan<- error) {
			_, err := io.Copy(serverConn, clientConn.NetConn())
			result <- err
		},
		streamToClient: func(clientConn *websocket.Conn, serverConn net.Conn, result chan<- error) {
			_, err := io.Copy(clientConn.NetConn(), serverConn)
			result <- err
		},
	}
}

func resolveUUIDFolder() (string, error) {
	files, err := os.ReadDir(baseFolder)
	if err != nil {
		return "", fmt.Errorf("error reading 'kubevirt-private' folder: %v", err)
	}
	pattern := "\\w{8}-\\w{4}-\\w{4}-\\w{4}-\\w{12}"
	r := regexp.MustCompile(pattern)
	for _, f := range files {
		if f.IsDir() && r.MatchString(f.Name()) {
			return f.Name(), nil
		}
	}
	return "", fmt.Errorf("could not find UUID directory")
}
