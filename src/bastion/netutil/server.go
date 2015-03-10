package netutil

import (
	"net"
	"sync"
)

type Server struct {
	*Listener
	AcceptListener    net.Listener
	ConnectionCounter uint64
	handlerMapMutex   sync.RWMutex
	RequestHandlers   map[string][]*RequestHandler
}

func NewDefaultServer(handler RequestHandler) *Server {
	server := &Server{}
	server.Listener = DefaultTcpListener(handler)
	server.ConnectionCounter = 0
	server.RequestHandlers = make(map[string][]*RequestHandler)
	return server
}

func (s *Server) AddHandler(cmdname string, handler *RequestHandler) {
	s.handlerMapMutex.Lock()
	defer s.handlerMapMutex.Unlock()
	s.RequestHandlers[cmdname] = append(s.RequestHandlers[cmdname], handler)
}

func (s *Server) GetHandlers(cmdname string) []*RequestHandler {
	s.handlerMapMutex.RLock()
	defer s.handlerMapMutex.RUnlock()
	return s.RequestHandlers[cmdname]
}

func (server *Server) NewReply(request *Request) (error, Reply) {
	reply := &Reply{}
	reply.RequestId = request.Id
	reply.Id = RequestId(nextRequestId())
	reply.Version = 1
	reply.Message = make(Message)
	return nil, Reply{}
}
