package godmin

import (
	"fmt"
	// "reflect"
	"testing"
)

type TestObject struct {
	Name     string
	Location *string
	Subs     []*TestObject
	Sub      *TestObject
}

func TestWwwForm(t *testing.T) {
	admin := NewModelAdmin("test", "test", nil, nil, nil, nil, nil, nil, nil)
	fmt.Println("testing www form Marshal")
	location := "Vancouver"
	obj := TestObject{"Obj", &location, nil, nil}
	obj2 := TestObject{"Obj2", &location, []*TestObject{&obj, &obj}, &obj}
	marshaled := Marshal(obj2, admin, "")
	fmt.Println("marshaled", marshaled)
}
