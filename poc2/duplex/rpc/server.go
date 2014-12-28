package rpc

import (
	"errors"
	"log"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Precompute the reflect type for error.  Can't use error directly
// because Typeof takes an empty interface value.  This is annoying.
var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

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

type service struct {
	name   string                      // name of service
	rcvr   reflect.Value               // receiver of methods for the service
	typ    reflect.Type                // type of the receiver
	method map[string]*reflectedMethod // registered methods
}

type reflectedMethod struct {
	Method       reflect.Method
	Channel      bool // if the method just takes a Channel object
	InputType    reflect.Type
	OutputType   reflect.Type
	OutputStream bool
	ContextType  reflect.Type
}

func (m *reflectedMethod) NewInput() reflect.Value {
	if m.InputType.Kind() == reflect.Ptr {
		return reflect.New(m.InputType.Elem())
	} else {
		return reflect.New(m.InputType)
	}
}

func (m *reflectedMethod) NewOutput() reflect.Value {
	return reflect.New(m.OutputType.Elem())
}

func (m *reflectedMethod) StreamingOuput() bool {
	return m.OutputType == typeOfSendStream
}

func (m *reflectedMethod) Call(receiver, input, output, context reflect.Value) []reflect.Value {
	if m.Channel {
		log.Panic("duplex: cannot call a method that takes a channel this way")
	}
	if m.InputType.Kind() != reflect.Ptr {
		input = input.Elem()
	}
	if m.ContextType != nil {
		return m.Method.Func.Call([]reflect.Value{receiver, context, input, output})
	} else {
		return m.Method.Func.Call([]reflect.Value{receiver, input, output})
	}
}

func newReflectedMethod(method reflect.Method) *reflectedMethod {
	mtype := method.Type
	mname := method.Name
	var channelType, outputType, inputType, contextType reflect.Type
	var stream, channel bool

	// Method must be exported.
	if method.PkgPath != "" {
		return nil
	}

	switch mtype.NumIn() {
	case 2:
		// channel method
		channelType = mtype.In(1)
	case 3:
		// normal method
		inputType = mtype.In(1)
		outputType = mtype.In(2)
		contextType = nil
	case 4:
		// method that takes a context
		inputType = mtype.In(2)
		outputType = mtype.In(3)
		contextType = mtype.In(1)
	default:
		log.Println("method", mname, "of", mtype, "has wrong number of ins:", mtype.NumIn())
		return nil
	}

	if channelType != nil {
		if channelType.Kind() != reflect.Ptr {
			log.Println("method", mname, "channel argument not a pointer:", channelType)
			return nil
		}
		if channelType != reflect.PtrTo(typeOfChannel) {
			log.Println("method", mname, "channel argument is", channelType, "not Channel")
			return nil
		}
		channel = true
	} else {

		// First arg need not be a pointer.
		if !isExportedOrBuiltinType(inputType) {
			log.Println(mname, "argument type not exported:", inputType)
			return nil
		}

		// the second argument will tell us if it's a streaming call
		// or a regular call
		if outputType == typeOfSendStream {
			// this is a streaming call
			stream = true
		} else if outputType.Kind() != reflect.Ptr {
			log.Println("method", mname, "reply type not a pointer:", outputType)
			return nil
		}

		// Reply type must be exported.
		if !isExportedOrBuiltinType(outputType) {
			log.Println("method", mname, "reply type not exported:", outputType)
			return nil
		}
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
	return &reflectedMethod{
		Method:       method,
		Channel:      channel,
		InputType:    inputType,
		OutputType:   outputType,
		OutputStream: stream,
		ContextType:  contextType,
	}
}

func (p *Peer) Register(rcvr interface{}) error {
	return p.register(rcvr, "", false)
}

func (p *Peer) RegisterName(name string, rcvr interface{}) error {
	return p.register(rcvr, name, true)
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
		s := "duplex register: type " + sname + " is not exported"
		log.Print(s)
		return errors.New(s)
	}
	if _, present := p.serviceMap[sname]; present {
		return errors.New("duplex: service already defined: " + sname)
	}
	s.name = sname
	s.method = make(map[string]*reflectedMethod)

	// Install the methods
	for m := 0; m < s.typ.NumMethod(); m++ {
		method := s.typ.Method(m)
		if rmethod := newReflectedMethod(method); rmethod != nil {
			if rmethod.ContextType != nil && rmethod.ContextType != reflect.PtrTo(p.contextType) {
				log.Println("method", method.Name, "has wrong context type:", rmethod.ContextType, "want", reflect.PtrTo(p.contextType))
				continue
			}
			s.method[method.Name] = rmethod
		}
	}

	if len(s.method) == 0 {
		s := "duplex register: type " + sname + " has no exported methods of suitable type"
		log.Print(s)
		return errors.New(s)
	}
	p.serviceMap[s.name] = s
	return nil
}

func errVal(returnVals []reflect.Value) (string, bool) {
	errInter := returnVals[0].Interface()
	errmsg := ""
	if errInter != nil {
		errmsg = errInter.(error).Error()
	}
	if errmsg != "" {
		return errmsg, true
	}
	return errmsg, false
}

func (p *Peer) Serve() {
	for {
		meta, ch := p.Accept()
		if meta == nil {
			return
		}

		serviceMethod := strings.Split(meta.Service(), ".")
		if len(serviceMethod) != 2 {
			ch.WriteError([]byte("duplex: service/method request ill-formed: " + meta.Service()))
			ch.Close()
			continue
		}
		p.serviceLock.Lock()
		service := p.serviceMap[serviceMethod[0]]
		p.serviceLock.Unlock()
		if service == nil {
			ch.WriteError([]byte("duplex: can't find service " + meta.Service()))
			ch.Close()
			continue
		}
		rmethod := service.method[serviceMethod[1]]
		if rmethod == nil {
			ch.WriteError([]byte("duplex: can't find method " + meta.Service()))
			ch.Close()
			continue
		}

		if rmethod.Channel {
			// method just takes a channel object
			if errmsg, err := errVal(rmethod.Method.Func.Call([]reflect.Value{service.rcvr, reflect.ValueOf(ch)})); err {
				ch.WriteError([]byte(errmsg))
			}
			ch.Close()
			continue
		}

		input := rmethod.NewInput()
		err := ch.ReadObject(input)
		if err != nil {
			log.Println("duplex: failure receiving", err)
			continue
		}

		contextv := reflect.New(p.contextType) // should optionally be passed in? https://github.com/flynn/rpcplus/blob/master/server.go#L562
		var output reflect.Value
		if rmethod.StreamingOuput() {
			output = reflect.ValueOf(SendStream{ch})
		} else {
			output = rmethod.NewOutput()
		}

		if errmsg, err := errVal(rmethod.Call(service.rcvr, input, output, contextv)); err {
			ch.WriteError([]byte(errmsg))
		} else {
			if !rmethod.StreamingOuput() {
				ch.WriteObject(output.Interface())
			}
		}
		ch.Close()
	}
}
