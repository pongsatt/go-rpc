package rpc

import (
	"bytes"
	"encoding/gob"
	"reflect"

	"github.com/pkg/errors"
)

// RPCdata represents the serializing format of structured data
type RPCdata struct {
	Name string        // name of the function
	Args []interface{} // request's or response's body expect error.
	Err  string        // Error any executing remote server
}

// Execute the given function if present
func Execute(req RPCdata, fFunc interface{}) ([]byte, error) {
	f := reflect.ValueOf(fFunc)

	// log.Printf("func %s is called\n", req.Name)

	// unpackage request arguments
	inArgs := make([]reflect.Value, len(req.Args))
	for i := range req.Args {
		inArgs[i] = reflect.ValueOf(req.Args[i])
	}

	// invoke requested method
	out := f.Call(inArgs)
	// now since we have followed the function signature style where last argument will be an error
	// so we will pack the response arguments expect error.
	resArgs := make([]interface{}, len(out)-1)
	for i := 0; i < len(out)-1; i++ {
		// Interface returns the constant value stored in v as an interface{}.
		resArgs[i] = out[i].Interface()
	}

	// pack error argument
	var er string
	if e, ok := out[len(out)-1].Interface().(error); ok {
		// convert the error into error string value
		er = e.Error()
	}

	respRPCData := RPCdata{Name: req.Name, Args: resArgs, Err: er}

	return Encode(respRPCData)
}

// Call makes rpc call
func Call(rpcName string, fPtr interface{}, rpcSend func(reqBytes []byte) ([]byte, error)) {
	container := reflect.ValueOf(fPtr).Elem()

	f := func(req []reflect.Value) []reflect.Value {
		errorHandler := func(err error) []reflect.Value {
			outArgs := make([]reflect.Value, container.Type().NumOut())
			for i := 0; i < len(outArgs)-1; i++ {
				outArgs[i] = reflect.Zero(container.Type().Out(i))
			}
			outArgs[len(outArgs)-1] = reflect.ValueOf(&err).Elem()
			return outArgs
		}

		// Process input parameters
		inArgs := make([]interface{}, 0, len(req))
		for _, arg := range req {
			inArgs = append(inArgs, arg.Interface())
		}

		// ReqRPC
		reqRPC := RPCdata{Name: rpcName, Args: inArgs}

		reqBytes, err := Encode(reqRPC)

		if err != nil {
			return errorHandler(err)
		}

		respBytes, err := rpcSend(reqBytes)

		if err != nil {
			return errorHandler(err)
		}

		rspRPC, err := Decode(respBytes)

		if err != nil {
			return errorHandler(err)
		}

		if rspRPC.Err != "" { // remote server error
			return errorHandler(errors.New(rspRPC.Err))
		}

		if len(rspRPC.Args) == 0 {
			rspRPC.Args = make([]interface{}, container.Type().NumOut())
		}
		// unpackage response arguments
		numOut := container.Type().NumOut()
		outArgs := make([]reflect.Value, numOut)
		for i := 0; i < numOut; i++ {
			if i != numOut-1 { // unpackage arguments (except error)
				if rspRPC.Args[i] == nil { // if argument is nil (gob will ignore "Zero" in transmission), set "Zero" value
					outArgs[i] = reflect.Zero(container.Type().Out(i))
				} else {
					outArgs[i] = reflect.ValueOf(rspRPC.Args[i])
				}
			} else { // unpackage error argument
				outArgs[i] = reflect.Zero(container.Type().Out(i))
			}
		}

		return outArgs
	}

	container.Set(reflect.MakeFunc(container.Type(), f))
}

// Encode The RPCdata in binary format which can
// be sent over the network.
func Encode(data RPCdata) ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode the binary data into the Go RPC struct
func Decode(b []byte) (RPCdata, error) {
	buf := bytes.NewBuffer(b)
	decoder := gob.NewDecoder(buf)
	var data RPCdata
	if err := decoder.Decode(&data); err != nil {
		return RPCdata{}, err
	}
	return data, nil
}
