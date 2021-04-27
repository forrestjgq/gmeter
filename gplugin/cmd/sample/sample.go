package main

import (
	"fmt"

	"github.com/forrestjgq/gmeter/gplugin"
)

type local struct {
}

func (l local) Recv(msg string) error {
	fmt.Println(msg)
	return nil
}

func Load(param string) (gplugin.MessageReceiver, error) {
	return local{}, nil
}

func main() {
	var c gplugin.ReceiverCreator
	c = Load
	c("")
}
