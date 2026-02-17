package protocol

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"net"
	"os"
	"sync"
)

type ClientConn struct {
	conn net.Conn
	mu   sync.Mutex
}

func (c *ClientConn) Send(event Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	c.conn.Write(append(data, '\n'))
}

type SocketServer struct {
	path    string
	clients []*ClientConn
	mu      sync.RWMutex
	cmdCh   chan Command
}

func NewSocketServer(path string) *SocketServer {
	return &SocketServer{
		path:  path,
		cmdCh: make(chan Command, 50),
	}
}

func (s *SocketServer) Start(ctx context.Context) {
	os.Remove(s.path)
	listener, err := net.Listen("unix", s.path)
	if err != nil {
		log.Fatalf("socket: failed to listen: %v", err)
	}
	defer listener.Close()
	defer os.Remove(s.path)

	log.Printf("socket: listening on %s", s.path)

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("socket: accept error: %v", err)
			continue
		}
		client := &ClientConn{conn: conn}
		s.mu.Lock()
		s.clients = append(s.clients, client)
		s.mu.Unlock()

		go s.handleClient(client)
	}
}

func (s *SocketServer) handleClient(client *ClientConn) {
	scanner := bufio.NewScanner(client.conn)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var cmd Command
		if err := json.Unmarshal(scanner.Bytes(), &cmd); err != nil {
			log.Printf("socket: invalid command: %v", err)
			continue
		}
		cmd.Conn = client
		s.cmdCh <- cmd
	}

	// Remove client on disconnect
	s.mu.Lock()
	for i, c := range s.clients {
		if c == client {
			s.clients = append(s.clients[:i], s.clients[i+1:]...)
			break
		}
	}
	s.mu.Unlock()
	client.conn.Close()
}

func (s *SocketServer) Broadcast(event Event) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, client := range s.clients {
		client.Send(event)
	}
}

func (s *SocketServer) Commands() <-chan Command {
	return s.cmdCh
}
