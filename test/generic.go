package main

import (
	"fmt"
)

type Acceptor[H any] struct {
}

func (a *Acceptor[H]) Hi() {
	h := new(H)
	h.Hi()
}

type Girl struct {
	name string
	age  int
}

func (g *Girl) Hi() {
	g.name = "lily"
	g.age = 20
	fmt.Printf("hi, i'm a girl, name:%s age:%d\n", g.name, g.age)
}

type Boy struct {
	name string
	age  int
}

func (b *Boy) Hi() {
	b.name = "lilei"
	b.age = 25
	fmt.Printf("hi, i'm a boy, name:%s age:%d\n", b.name, b.age)
}

func main() {
	girlAcceptor := Acceptor[Girl]{}
	girlAcceptor.Hi()

}
