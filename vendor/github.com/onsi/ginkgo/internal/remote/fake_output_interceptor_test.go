package remote_test

import "os"

type fakeOutputInterceptor struct {
	DidStartInterceptingOutput bool
	DidStopInterceptingOutput  bool
	InterceptedOutput          string
}

func (interceptor *fakeOutputInterceptor) StartInterceptingOutput() error {
	interceptor.DidStartInterceptingOutput = true
	return nil
}

func (interceptor *fakeOutputInterceptor) StopInterceptingAndReturnOutput() (string, error) {
	interceptor.DidStopInterceptingOutput = true
	return interceptor.InterceptedOutput, nil
}

func (interceptor *fakeOutputInterceptor) StreamTo(*os.File) {
}
