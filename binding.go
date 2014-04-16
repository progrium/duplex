package duplex

import (
	"errors"
	"io"
	"log"
	"reflect"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"./dpx"
)

// This is the high level API in Go that mostly adds language specific
// conveniences around the core Duplex API that would eventually be in C

// Precompute the reflect type for error.  Can't use error directly
// because Typeof takes an empty interface value.  This is annoying.
var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

var typeOfSendStream = reflect.TypeOf(SendStream{})
var typeOfReceiveStream = reflect.TypeOf(ReceiveStream{})

type service struct {
	name   string                 // name of service
	rcvr   reflect.Value          // receiver of methods for the service
	typ    reflect.Type           // type of the receiver
	method map[string]*methodType // registered methods
}

type methodType struct {
	sync.Mutex  // protects counters
	method      reflect.Method
	ArgType     reflect.Type
	ReplyType   reflect.Type
	ContextType reflect.Type
	stream      bool
	numCalls    uint
}

func (m *methodType) TakesContext() bool {
	return m.ContextType != nil
}

type Peer struct {
	P *dpx.Peer

	serviceLock sync.Mutex
	serviceMap  map[string]*service
	contextType reflect.Type
}

func NewPeer() *Peer {
	p := new(Peer)
	p.P = dpx.NewPeer()
	p.contextType = reflect.TypeOf("")
	p.serviceMap = make(map[string]*service)
	return p
}

func (p *Peer) Bind(addr string) error {
	return dpx.Bind(p.P, addr)
}

func (p *Peer) Connect(addr string) error {
	return dpx.Connect(p.P, addr)
}

func (p *Peer) Close() error {
	return dpx.Close(p.P)
}

func (p *Peer) Accept() (string, *Channel) {
	method, ch := dpx.Accept(p.P)
	return method, &Channel{C: ch}
}

func (p *Peer) Serve() {
	for {
		method, ch := p.Accept()

		serviceMethod := strings.Split(method, ".")
		if len(serviceMethod) != 2 {
			log.Println("duplex: service/method request ill-formed: " + method)
			continue
		}
		p.serviceLock.Lock()
		service := p.serviceMap[serviceMethod[0]]
		p.serviceLock.Unlock()
		if service == nil {
			log.Println("duplex: can't find service " + method)
			continue
		}
		mtype := service.method[serviceMethod[1]]
		if mtype == nil {
			log.Println("duplex: can't find method " + method)
			continue
		}

		var argv reflect.Value
		// Decode the argument value.
		argIsValue := false // if true, need to indirect before calling.
		if mtype.ArgType.Kind() == reflect.Ptr {
			argv = reflect.New(mtype.ArgType.Elem())
		} else {
			argv = reflect.New(mtype.ArgType)
			argIsValue = true
		}

		err := ch.Receive(argv)
		if err != nil {
			log.Println("duplex: failure receiving " + err.Error())
		}
		if argIsValue {
			argv = argv.Elem()
		}

		function := mtype.method.Func
		contextVal := reflect.New(p.contextType) // should optionally be passed in? https://github.com/flynn/rpcplus/blob/master/server.go#L562
		var returnValues []reflect.Value
		if !mtype.stream {
			replyv := reflect.New(mtype.ReplyType.Elem())

			// Invoke the method, providing a new value for the reply.
			if mtype.TakesContext() {
				returnValues = function.Call([]reflect.Value{service.rcvr, contextVal, argv, replyv})
			} else {
				returnValues = function.Call([]reflect.Value{service.rcvr, argv, replyv})
			}

			// The return value for the method is an error.
			errInter := returnValues[0].Interface()
			errmsg := ""
			if errInter != nil {
				errmsg = errInter.(error).Error()
			}
			if errmsg != "" {
				// TODO: handle if errmsg != ""
			}

			ch.Send(replyv.Interface())
			continue
		}

		sendChan := make(chan interface{})
		errChan := make(chan error)
		stream := reflect.ValueOf(SendStream{ch, sendChan, errChan})
		funcDone := make(chan struct{})
		sendDone := make(chan struct{})

		var streamErr error
		go func() {
			defer close(sendDone)
			for {
				// TODO handle CloseStream
				select {
				case obj := <-sendChan:
					streamErr = ch.Send(obj)
					if streamErr != nil {
						errChan <- streamErr
						return
					}
				case <-funcDone:
					return
				}
			}
		}()

		// Invoke the method, providing a new value for the reply.
		if mtype.TakesContext() {
			returnValues = function.Call([]reflect.Value{service.rcvr, contextVal, argv, stream})
		} else {
			returnValues = function.Call([]reflect.Value{service.rcvr, argv, stream})
		}
		close(funcDone)
		<-sendDone

		errInter := returnValues[0].Interface()
		errmsg := ""
		if errInter != nil {
			// the function returned an error, we use that
			errmsg = errInter.(error).Error()
		} else if streamErr != nil {
			errmsg = streamErr.Error()
		}
		if errmsg != "" {

		}
		ch.SendLast(nil)
	}
}

func (p *Peer) Register(rcvr interface{}) error {
	return p.register(rcvr, "", false)
}

func (p *Peer) RegisterName(name string, rcvr interface{}) error {
	return p.register(rcvr, name, true)
}

func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return isExported(t.Name()) || t.PkgPath() == ""
}

// prepareMethod returns a methodType for the provided method or nil
// in case if the method was unsuitable.
func prepareMethod(method reflect.Method) *methodType {
	mtype := method.Type
	mname := method.Name
	var replyType, argType, contextType reflect.Type

	stream := false
	// Method must be exported.
	if method.PkgPath != "" {
		return nil
	}

	switch mtype.NumIn() {
	case 3:
		// normal method
		argType = mtype.In(1)
		replyType = mtype.In(2)
		contextType = nil
	case 4:
		// method that takes a context
		argType = mtype.In(2)
		replyType = mtype.In(3)
		contextType = mtype.In(1)
	default:
		log.Println("method", mname, "of", mtype, "has wrong number of ins:", mtype.NumIn())
		return nil
	}

	// First arg need not be a pointer.
	if !isExportedOrBuiltinType(argType) {
		log.Println(mname, "argument type not exported:", argType)
		return nil
	}

	// the second argument will tell us if it's a streaming call
	// or a regular call
	if replyType == typeOfSendStream {
		// this is a streaming call
		stream = true
	} else if replyType.Kind() != reflect.Ptr {
		log.Println("method", mname, "reply type not a pointer:", replyType)
		return nil
	}

	// Reply type must be exported.
	if !isExportedOrBuiltinType(replyType) {
		log.Println("method", mname, "reply type not exported:", replyType)
		return nil
	}
	// Method needs one out.
	if mtype.NumOut() != 1 {
		log.Println("method", mname, "has wrong number of outs:", mtype.NumOut())
		return nil
	}
	// The return type of the method must be error.
	if returnType := mtype.Out(0); returnType != typeOfError {
		log.Println("method", mname, "returns", returnType.String(), "not error")
		return nil
	}
	return &methodType{method: method, ArgType: argType, ReplyType: replyType, ContextType: contextType, stream: stream}
}

func (p *Peer) register(rcvr interface{}, name string, useName bool) error {
	p.serviceLock.Lock()
	defer p.serviceLock.Unlock()
	if p.serviceMap == nil {
		p.serviceMap = make(map[string]*service)
	}
	s := new(service)
	s.typ = reflect.TypeOf(rcvr)
	s.rcvr = reflect.ValueOf(rcvr)
	sname := reflect.Indirect(s.rcvr).Type().Name()
	if useName {
		sname = name
	}
	if sname == "" {
		log.Fatal("duplex: no service name for type", s.typ.String())
	}
	if !isExported(sname) && !useName {
		s := "duplex Register: type " + sname + " is not exported"
		log.Print(s)
		return errors.New(s)
	}
	if _, present := p.serviceMap[sname]; present {
		return errors.New("rpc: service already defined: " + sname)
	}
	s.name = sname
	s.method = make(map[string]*methodType)

	// Install the methods
	for m := 0; m < s.typ.NumMethod(); m++ {
		method := s.typ.Method(m)
		if mt := prepareMethod(method); mt != nil {
			if mt.ContextType != nil && mt.ContextType != reflect.PtrTo(p.contextType) {
				log.Println("method", method.Name, "has wrong context type:", mt.ContextType, "want", reflect.PtrTo(p.contextType))
				continue
			}
			s.method[method.Name] = mt
		}
	}

	if len(s.method) == 0 {
		s := "duplex Register: type " + sname + " has no exported methods of suitable type"
		log.Print(s)
		return errors.New(s)
	}
	p.serviceMap[s.name] = s
	return nil
}

type Channel struct {
	C *dpx.Channel
}

func (c *Channel) Send(obj interface{}) error {
	return dpx.Send(c.C, obj)
}

func (c *Channel) SendLast(obj interface{}) error {
	return dpx.SendLast(c.C, obj)
}

func (c *Channel) Receive(obj interface{}) error {
	return dpx.Receive(c.C, obj)
}

// ServerError represents an error that has been returned from
// the remote side of the RPC connection.
type ServerError string

func (e ServerError) Error() string {
	return string(e)
}

type SendStream struct {
	channel *Channel
	Send    chan<- interface{}
	Error   <-chan error
}

type ReceiveStream struct {
	channel *Channel
	Receive <-chan interface{}
}

func (s *ReceiveStream) Close() error {
	return dpx.SendErr(s.channel.C, "CloseStream")
}

type Call struct {
	ServiceMethod string      // The name of the service and method to call.
	Args          interface{} // The argument to the function (*struct).
	Reply         interface{} // The reply from the function (*struct for single, chan * struct for streaming).
	Error         error       // After completion, the error status.
	Done          chan *Call  // Strobes when call is complete (nil for streaming RPCs)
	Stream        bool        // True for a streaming RPC call, false otherwise

	channel *dpx.Channel
	peer    *Peer
}

func (c *Call) CloseStream() error {
	if !c.Stream {
		return errors.New("duplex: cannot close non-stream request")
	}

	// TODO send close stream control frame
	return nil
}

func (c *Call) done() {
	if c.Stream {
		// need to close the channel. Client won't be able to read any more.
		reflect.ValueOf(c.Reply).Close()
		return
	}

	select {
	case c.Done <- c:
		// ok
	default:
		// We don't want to block here.  It is the caller's responsibility to make
		// sure the channel has enough buffer space. See comment in Go().
		log.Println("duplex: discarding Call reply due to insufficient Done chan capacity")
	}
}

// Go invokes the function asynchronously.  It returns the Call structure representing
// the invocation.  The done channel will signal when the call is complete by returning
// the same Call object.  If done is nil, Go will allocate a new channel.
// If non-nil, done must be buffered or Go will deliberately crash.
func (peer *Peer) Go(serviceMethod string, args interface{}, reply interface{}, done chan *Call) *Call {
	call := new(Call)
	call.ServiceMethod = serviceMethod
	call.Args = args
	call.Reply = reply
	if done == nil {
		done = make(chan *Call, 10) // buffered.
	} else {
		// If caller passes done != nil, it must arrange that
		// done has enough buffer for the number of simultaneous
		// RPCs that will be using that channel.  If the channel
		// is totally unbuffered, it's best not to run at all.
		if cap(done) == 0 {
			log.Panic("duplex: done channel is unbuffered")
		}
	}
	call.Done = done
	call.peer = peer
	call.channel = dpx.Open(peer.P, call.ServiceMethod)
	dpx.SendLast(call.channel, call.Args)
	go func() {
		defer call.done()
		err := dpx.Receive(call.channel, call.Reply)
		if err != nil && err != io.EOF {
			// TODO handle non remote/parsing error
			call.Error = ServerError(err.Error())
		}
	}()
	return call
}

// Go invokes the streaming function asynchronously.  It returns the Call structure representing
// the invocation.
func (peer *Peer) StreamGo(serviceMethod string, args interface{}, replyStream interface{}) *Call {
	// first check the replyStream object is a stream of pointers to a data structure
	typ := reflect.TypeOf(replyStream)
	// FIXME: check the direction of the channel, maybe?
	if typ.Kind() != reflect.Chan || typ.Elem().Kind() != reflect.Ptr {
		log.Panic("duplex: replyStream is not a channel of pointers")
		return nil
	}

	call := new(Call)
	call.ServiceMethod = serviceMethod
	call.Args = args
	call.Reply = replyStream
	call.Stream = true
	call.peer = peer
	call.channel = dpx.Open(peer.P, call.ServiceMethod)
	dpx.SendLast(call.channel, call.Args)
	go func() {
		defer call.done()
		for {
			// call.Reply is a chan *T2
			// we need to create a T2 and get a *T2 back
			value := reflect.New(reflect.TypeOf(call.Reply).Elem().Elem()).Interface()
			err := dpx.Receive(call.channel, value)
			if err != nil {
				if err != io.EOF {
					call.Error = ServerError(err.Error())
				}
				return
			} else {
				if value != nil {
					// writing on the channel could block forever. For
					// instance, if a client calls 'close', this might block
					// forever.  the current suggestion is for the
					// client to drain the receiving channel in that case
					reflect.ValueOf(call.Reply).Send(reflect.ValueOf(value))
				} else {
					return
				}
			}
		}
	}()
	return call
}

// Call invokes the named function, waits for it to complete, and returns its error status.
func (peer *Peer) Call(serviceMethod string, args interface{}, reply interface{}) error {
	call := <-peer.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
	return call.Error
}
