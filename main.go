package main

import (
	"fmt"
	"github.com/sjtug/lug/config"
	"github.com/sjtug/lug/manager"
	"github.com/sjtug/lug/worker"
)

func main() {
	config.Foo()
	manager.Foo()
	worker.Foo()
	fmt.Printf("Hello world!")
}
