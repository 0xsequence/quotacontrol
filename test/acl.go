package test

import (
	"errors"
	"reflect"

	"github.com/0xsequence/quotacontrol/middleware"
)

func VerifyACL[T any](acl middleware.ACL) error {
	var t T
	iType := reflect.TypeOf(&t).Elem()
	service := iType.Name()
	m, ok := acl[service]
	if !ok {
		return errors.New("service " + service + " not found")
	}
	var errList []error
	for i := 0; i < iType.NumMethod(); i++ {
		method := iType.Method(i)
		if _, ok := m[method.Name]; !ok {
			errList = append(errList, errors.New(""+service+"."+method.Name+" not found"))
		}
	}
	return errors.Join(errList...)
}
