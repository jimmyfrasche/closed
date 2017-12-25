// +build ignore

package testdata

var i int

type t int

//TODO need a case where an enum is part of an iface sum

type Enum int

type Enum2 = Enum

const (
	A Enum = iota
	B
	C

	X = B

	//not enums
	U string  = "str"
	W float64 = 3.14
	Y         = 1
	Z rune    = 'c'
)

type Bitset uint8

const (
	F0 Bitset = 1 << iota
	F1
	F2

	G0 = F0 | F2
)

type Sum interface {
	sum()
}

type Int interface {
	notSum(int)
}

type sum struct{}

func (sum) sum() {}

type (
	D struct {
		sum
	}

	E struct {
		sum
	}

	F struct{}

	G = F
)

func (F) sum() {}

func (F) String() string {
	return ""
}
