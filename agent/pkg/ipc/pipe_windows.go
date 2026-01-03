//go:build windows
// +build windows

package ipc

import (
        "bufio"
        "encoding/json"
        "fmt"
        "net"
        "sync"
        "time"

        "github.com/Microsoft/go-winio"
        "github.com/ctolnik/Office-Monitor/agent/pkg/logger"
        "go.uber.org/zap"
)

type PipeServer struct {
        pipeName    string
        listener    net.Listener
        handlers    map[EventType]func(Event) error
        mu          sync.RWMutex
        stopChan    chan struct{}
        wg          sync.WaitGroup
        log         *zap.Logger
}

func NewPipeServer(pipeName string) *PipeServer {
        return &PipeServer{
                pipeName: pipeName,
                handlers: make(map[EventType]func(Event) error),
                stopChan: make(chan struct{}),
                log:      logger.WithComponent("ipc_server"),
        }
}

func (s *PipeServer) RegisterHandler(eventType EventType, handler func(Event) error) {
        s.mu.Lock()
        defer s.mu.Unlock()
        s.handlers[eventType] = handler
}

func (s *PipeServer) Start() error {
        listener, err := winio.ListenPipe(s.pipeName, &winio.PipeConfig{
                SecurityDescriptor: "",
                MessageMode:        false,
                InputBufferSize:    65536,
                OutputBufferSize:   65536,
        })
        if err != nil {
                return fmt.Errorf("failed to create pipe: %w", err)
        }

        s.listener = listener
        s.log.Info("Pipe server started", zap.String("pipe", s.pipeName))

        s.wg.Add(1)
        go s.acceptLoop()

        return nil
}

func (s *PipeServer) acceptLoop() {
        defer s.wg.Done()

        for {
                select {
                case <-s.stopChan:
                        return
                default:
                }

                conn, err := s.listener.Accept()
                if err != nil {
                        select {
                        case <-s.stopChan:
                                return
                        default:
                                s.log.Error("Accept failed", zap.Error(err))
                                continue
                        }
                }

                s.wg.Add(1)
                go s.handleConnection(conn)
        }
}

func (s *PipeServer) handleConnection(conn net.Conn) {
        defer s.wg.Done()
        defer conn.Close()

        scanner := bufio.NewScanner(conn)
        scanner.Buffer(make([]byte, 65536), 65536)

        for scanner.Scan() {
                var event Event
                if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
                        s.log.Error("Failed to parse event", zap.Error(err))
                        continue
                }

                s.mu.RLock()
                handler, ok := s.handlers[event.Type]
                s.mu.RUnlock()

                if ok {
                        if err := handler(event); err != nil {
                                s.log.Error("Handler failed",
                                        zap.String("event_type", string(event.Type)),
                                        zap.Error(err))
                        }
                }
        }
}

func (s *PipeServer) Stop() {
        close(s.stopChan)
        if s.listener != nil {
                s.listener.Close()
        }
        s.wg.Wait()
        s.log.Info("Pipe server stopped")
}

type PipeClient struct {
        pipeName     string
        conn         net.Conn
        mu           sync.Mutex
        reconnecting bool
        stopChan     chan struct{}
        log          *zap.Logger
}

func NewPipeClient(pipeName string) *PipeClient {
        return &PipeClient{
                pipeName: pipeName,
                stopChan: make(chan struct{}),
                log:      logger.WithComponent("ipc_client"),
        }
}

func (c *PipeClient) Connect() error {
        c.mu.Lock()
        defer c.mu.Unlock()

        if c.conn != nil {
                return nil
        }

        conn, err := winio.DialPipe(c.pipeName, nil)
        if err != nil {
                return fmt.Errorf("failed to connect to pipe: %w", err)
        }

        c.conn = conn
        c.log.Info("Connected to pipe", zap.String("pipe", c.pipeName))
        return nil
}

func (c *PipeClient) Send(event Event) error {
        c.mu.Lock()
        defer c.mu.Unlock()

        if c.conn == nil {
                return fmt.Errorf("not connected")
        }

        data, err := json.Marshal(event)
        if err != nil {
                return err
        }

        data = append(data, '\n')
        c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
        _, err = c.conn.Write(data)
        if err != nil {
                c.conn.Close()
                c.conn = nil
                return err
        }

        return nil
}

func (c *PipeClient) Close() {
        c.mu.Lock()
        defer c.mu.Unlock()

        close(c.stopChan)
        if c.conn != nil {
                c.conn.Close()
                c.conn = nil
        }
        c.log.Info("Pipe client closed")
}
