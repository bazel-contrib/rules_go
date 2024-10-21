package usesyso

//go:noescape
func foo() int32

func Foo() int32 {
	return foo()
}
