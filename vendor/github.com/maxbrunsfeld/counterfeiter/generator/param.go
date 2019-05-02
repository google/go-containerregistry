package generator

import "strings"

type Params []Param

type Param struct {
	Name       string
	Type       string
	IsVariadic bool
	IsSlice    bool
}

func (p Params) Slices() Params {
	var result Params
	for i := range p {
		if p[i].IsSlice {
			result = append(result, p[i])
		}
	}
	return result
}

func (p Params) HasLength() bool {
	return len(p) > 0
}

func (p Params) WithPrefix(prefix string) string {
	if len(p) == 0 {
		return ""
	}

	params := []string{}
	for i := range p {
		if prefix == "" {
			params = append(params, unexport(p[i].Name))
		} else {
			params = append(params, prefix+unexport(p[i].Name))
		}
	}
	return strings.Join(params, ", ")
}

func (p Params) AsArgs() string {
	if len(p) == 0 {
		return ""
	}

	params := []string{}
	for i := range p {
		params = append(params, p[i].Type)
	}
	return strings.Join(params, ", ")
}

func (p Params) AsNamedArgsWithTypes() string {
	if len(p) == 0 {
		return ""
	}

	params := []string{}
	for i := range p {
		params = append(params, unexport(p[i].Name)+" "+p[i].Type)
	}
	return strings.Join(params, ", ")
}

func (p Params) AsNamedArgs() string {
	if len(p) == 0 {
		return ""
	}

	params := []string{}
	for i := range p {
		if p[i].IsSlice {
			params = append(params, unexport(p[i].Name)+"Copy")
		} else {
			params = append(params, unexport(p[i].Name))
		}
	}
	return strings.Join(params, ", ")
}

func (p Params) AsNamedArgsForInvocation() string {
	if len(p) == 0 {
		return ""
	}

	params := []string{}
	for i := range p {
		if p[i].IsVariadic {
			params = append(params, unexport(p[i].Name)+"...")
		} else {
			params = append(params, unexport(p[i].Name))
		}
	}
	return strings.Join(params, ", ")
}

func (p Params) AsReturnSignature() string {
	if len(p) == 0 {
		return ""
	}
	if len(p) == 1 {
		if p[0].IsVariadic {
			return strings.Replace(p[0].Type, "...", "[]", -1)
		}
		return p[0].Type
	}
	result := "("
	for i := range p {
		t := p[i].Type
		if p[i].IsVariadic {
			t = strings.Replace(t, "...", "[]", -1)
		}
		result = result + t
		if i < len(p) {
			result = result + ", "
		}
	}
	result = result + ")"
	return result
}
