package main

import "fmt"

type Ai interface {
	Say()
}

var aii Ai

type A struct {
	name string
}

func (a *A) Open() {
	a.name = "jack"
	aii = a
}
func (a *A) Say() {
	fmt.Println("i'm A")
}

type A1 struct {
	A
}

func (a *A1) Say() {
	fmt.Println("i'm ", a.name)
}

func Say(ai Ai) {
	ai.Say()
}
func main() {
	a1 := A1{}
	a1.Open() // aii = a
	Say(&a1)
	aii.Say() // go 没有多态
}
