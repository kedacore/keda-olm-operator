package controller

import (
	"github.com/kedacore/keda-olm-operator/pkg/controller/kedacontroller"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, kedacontroller.Add)
}
