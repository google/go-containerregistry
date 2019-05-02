package generator

import (
	"errors"
	"go/types"
)

func (f *Fake) loadMethodForFunction() error {
	t, ok := f.Target.Type().(*types.Named)
	if !ok {
		return errors.New("target is not a named type")
	}
	sig, ok := t.Underlying().(*types.Signature)
	if !ok {
		return errors.New("target does not have an underlying function signature")
	}
	f.addTypesForMethod(sig)
	importsMap := f.importsMap()
	function := methodForSignature(sig, f.Name, f.TargetAlias, f.TargetName, importsMap)
	f.Function = function
	return nil
}
