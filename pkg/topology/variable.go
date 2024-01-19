package topology

import (
	"fmt"
	"regexp"
)

// Variable
const (
	REGEX_VARIABLE = `\${([^${}]+)}` // ${var_name}
)

type Variable struct {
	Name        string
	Description string
	Value       string
	Resolved    bool
}

type Variables struct {
	m map[string]*Variable
	r *regexp.Regexp
}

func NewVariables() *Variables {
	return &Variables{
		m: map[string]*Variable{},
	}
}

func (vars *Variables) Register(v Variable) error {
	name := v.Name
	if _, ok := vars.m[name]; ok {
		return fmt.Errorf("variable '%s' duplicate define", name)
	}

	vars.m[name] = &v
	return nil
}

func (vars *Variables) Get(name string) (string, error) {
	v, ok := vars.m[name]
	if !ok {
		return "", fmt.Errorf("variable '%s' not found", name)
	} else if !v.Resolved {
		return "", fmt.Errorf("variable '%s' unresolved", name)
	}

	return v.Value, nil
}

func (vars *Variables) Set(name, value string) error {
	v, ok := vars.m[name]
	if !ok {
		return fmt.Errorf("variable '%s' not found", name)
	}

	v.Value = value
	v.Resolved = true
	return nil
}

func (vars *Variables) resolve(name string, marked map[string]bool) (string, error) {
	marked[name] = true
	v, ok := vars.m[name]
	if !ok {
		return "", fmt.Errorf("variable '%s' not defined", name)
	} else if v.Resolved {
		return v.Value, nil
	}

	matches := vars.r.FindAllStringSubmatch(v.Value, -1)
	if len(matches) == 0 { // no variable
		v.Resolved = true
		return v.Value, nil
	}

	// resolve all sub-variable
	for _, mu := range matches {
		name = mu[1]
		if _, err := vars.resolve(name, marked); err != nil {
			return "", err
		}
	}

	// ${var}
	v.Value = vars.r.ReplaceAllStringFunc(v.Value, func(name string) string {
		return vars.m[name[2:len(name)-1]].Value
	})
	v.Resolved = true
	return v.Value, nil
}

func (vars *Variables) Build() error {
	r, err := regexp.Compile(REGEX_VARIABLE)
	if err != nil {
		return err
	}

	vars.r = r
	for _, v := range vars.m {
		marked := map[string]bool{}
		if _, err := vars.resolve(v.Name, marked); err != nil {
			return err
		}
	}
	return nil
}

// "hello, ${varname}" => "hello, world"
func (vars *Variables) Rendering(s string) (string, error) {
	matches := vars.r.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 { // no variable
		return s, nil
	}

	var err error
	value := vars.r.ReplaceAllStringFunc(s, func(name string) string {
		val, e := vars.Get(name[2 : len(name)-1])
		if e != nil && err == nil {
			err = e
		}
		return val
	})
	return value, err
}
