package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"

	commonv1pb "github.com/dapr/dapr/pkg/proto/common/v1"
	runtimev1pb "github.com/dapr/dapr/pkg/proto/runtime/v1"
	"github.com/dapr/dapr/tests/apps/utils"
	"github.com/dapr/kit/ptr"
)

var (
	appPort          string
	appProtocol      string
	controlPort      string
	daprPort         string
	healthCheckPlan  *healthCheck
	lastInputBinding = newCountAndLast()
	lastHealthCheck  = newCountAndLast()
	ready            chan struct{}
	httpClient       = utils.NewHTTPClient()
)

const invokeUrl = "http://localhost:%s/v1.0/invoke/%s/method/%s"

func main() {
	ready = make(chan struct{})
	healthCheckPlan = newHealthCheck()

	appPort = os.Getenv("APP_PORT")
	if appPort == "" {
		appPort = "4000"
	}

	appProtocol = os.Getenv("APP_PROTOCOL")

	controlPort = os.Getenv("CONTROL_PORT")
	if controlPort == "" {
		controlPort = "3000"
	}

	daprPort = os.Getenv("DAPR_HTTP_PORT")

	// Start the control server
	go startControlServer()

	if appProtocol == "grpc" {
		// Blocking call
		startGrpc()
	} else {
		// Blocking call
		startHttp()
	}
}

func startGrpc() {
	lis, err := net.Listen("tcp", "0.0.0.0:"+appPort)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	h := &grpcServer{}
	s := grpc.NewServer()
	runtimev1pb.RegisterAppCallbackServer(s, h)
	runtimev1pb.RegisterAppCallbackHealthCheckServer(s, h)

	// Stop the gRPC server when we get a termination signal
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		// Wait for cancelation signal
		<-stopCh
		log.Println("Shutdown signal received")
		s.GracefulStop()
	}()

	// Blocking call
	log.Printf("Health App GRPC server listening on :%s", appPort)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to start gRPC server: %v", err)
	}
	log.Println("App shut down")
}

type countAndLast struct {
	count    int64
	lastTime time.Time
	lock     *sync.Mutex
}

func newCountAndLast() *countAndLast {
	return &countAndLast{
		lock: &sync.Mutex{},
	}
}

func (c *countAndLast) Record() {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.count++
	c.lastTime = time.Now()
}

func (c *countAndLast) MarshalJSON() ([]byte, error) {
	obj := struct {
		// Total number of actions
		Count int64 `json:"count"`
		// Time since last action, in ms
		Last *int64 `json:"last"`
	}{
		Count: c.count,
	}
	if c.lastTime.Unix() > 0 {
		obj.Last = ptr.Of(time.Now().Sub(c.lastTime).Milliseconds())
	}
	return json.Marshal(obj)
}

func startControlServer() {
	// Wait until the first health probe
	log.Print("Waiting for signalto start control server…")
	<-ready

	port, _ := strconv.Atoi(controlPort)
	log.Printf("Health App control server listening on http://:%d", port)
	utils.StartServer(port, func() *mux.Router {
		r := mux.NewRouter().StrictSlash(true)

		// Log requests and their processing time
		r.Use(utils.LoggerMiddleware)

		// Hello world
		r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("⚙️"))
		}).Methods("GET")

		// Get last input binding
		r.HandleFunc("/last-input-binding", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			j, _ := json.Marshal(lastInputBinding)
			w.Write(j)
		}).Methods("GET")

		// Get last health check
		r.HandleFunc("/last-health-check", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			j, _ := json.Marshal(lastHealthCheck)
			w.Write(j)
		}).Methods("GET")

		// Performs a service invocation
		r.HandleFunc("/invoke/{name}/{method}", func(w http.ResponseWriter, r *http.Request) {
			vars := mux.Vars(r)
			u := fmt.Sprintf(invokeUrl, daprPort, vars["name"], vars["method"])
			log.Println("Invoking URL", u)
			res, err := httpClient.Post(u, r.Header.Get("content-type"), r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Failed to invoke method: " + err.Error()))
				return
			}
			defer res.Body.Close()

			w.WriteHeader(res.StatusCode)
			io.Copy(w, res.Body)
		}).Methods("POST")

		// Update the plan
		r.HandleFunc("/set-plan", func(w http.ResponseWriter, r *http.Request) {
			ct := r.Header.Get("content-type")
			if ct != "application/json" && !strings.HasPrefix(ct, "application/json;") {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Invalid content type"))
				return
			}

			plan := []bool{}
			err := json.NewDecoder(r.Body).Decode(&plan)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Failed to parse body: " + err.Error()))
				return
			}

			healthCheckPlan.SetPlan(plan)

			w.WriteHeader(http.StatusNoContent)
		}).Methods("POST", "PUT")

		return r
	}, false)
}

func startHttp() {
	port, _ := strconv.Atoi(appPort)
	log.Printf("Health App HTTP server listening on http://:%d", port)
	utils.StartServer(port, httpRouter, true)
}

func httpRouter() *mux.Router {
	r := mux.NewRouter().StrictSlash(true)

	// Log requests and their processing time
	r.Use(utils.LoggerMiddleware)

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("👋"))
	}).Methods("GET")

	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if ready != nil {
			close(ready)
			ready = nil
		}

		lastHealthCheck.Record()

		err := healthCheckPlan.Do()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}).Methods("GET")

	r.HandleFunc("/schedule", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Received scheduled message")
		lastInputBinding.Record()
		w.WriteHeader(http.StatusOK)
	}).Methods("POST")

	r.HandleFunc("/foo", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Received foo request")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("🤗"))
	}).Methods("POST")

	return r
}

// Server for gRPC
type grpcServer struct {
}

func (s *grpcServer) HealthCheck(ctx context.Context, _ *emptypb.Empty) (*runtimev1pb.HealthCheckResponse, error) {
	if ready != nil {
		close(ready)
		ready = nil
	}

	lastHealthCheck.Record()

	err := healthCheckPlan.Do()
	if err != nil {
		return nil, err
	}
	return &runtimev1pb.HealthCheckResponse{}, nil
}

func (s *grpcServer) OnInvoke(_ context.Context, in *commonv1pb.InvokeRequest) (*commonv1pb.InvokeResponse, error) {
	if in.Method == "foo" {
		log.Println("Received method invocation: " + in.Method)
		return &commonv1pb.InvokeResponse{
			Data: &anypb.Any{
				Value: []byte("🤗"),
			},
		}, nil
	}

	return nil, errors.New("unexpected method invocation: " + in.Method)
}

func (s *grpcServer) ListTopicSubscriptions(_ context.Context, in *emptypb.Empty) (*runtimev1pb.ListTopicSubscriptionsResponse, error) {
	return &runtimev1pb.ListTopicSubscriptionsResponse{
		Subscriptions: nil,
	}, nil
}

func (s *grpcServer) OnTopicEvent(_ context.Context, in *runtimev1pb.TopicEventRequest) (*runtimev1pb.TopicEventResponse, error) {
	return &runtimev1pb.TopicEventResponse{}, nil
}

func (s *grpcServer) ListInputBindings(_ context.Context, in *emptypb.Empty) (*runtimev1pb.ListInputBindingsResponse, error) {
	return &runtimev1pb.ListInputBindingsResponse{
		Bindings: []string{"schedule"},
	}, nil
}

func (s *grpcServer) OnBindingEvent(_ context.Context, in *runtimev1pb.BindingEventRequest) (*runtimev1pb.BindingEventResponse, error) {
	if in.Name == "schedule" {
		log.Println("Received binding event: " + in.Name)
		lastInputBinding.Record()
		return &runtimev1pb.BindingEventResponse{}, nil
	}

	return nil, errors.New("unexpected binding event: " + in.Name)
}

type healthCheck struct {
	plan  []bool
	count int
	lock  *sync.Mutex
}

func newHealthCheck() *healthCheck {
	return &healthCheck{
		lock: &sync.Mutex{},
	}
}

func (h *healthCheck) SetPlan(plan []bool) {
	h.lock.Lock()
	defer h.lock.Unlock()

	// Set the new plan and reset the counter
	h.plan = plan
	h.count = 0
}

func (h *healthCheck) Do() error {
	h.lock.Lock()
	defer h.lock.Unlock()

	success := true
	if h.count < len(h.plan) {
		success = h.plan[h.count]
		h.count++
	}

	if success {
		log.Println("Responding to health check request with success")
		return nil
	} else {
		log.Println("Responding to health check request with failure")
		return errors.New("simulated failure")
	}
}
