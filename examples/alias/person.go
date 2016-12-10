package alias

// Person :
type Person struct {
	Name string
}

type P *Person
type PS []Person
type PS2 []P
type PSP *[]P
