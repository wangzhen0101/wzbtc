package main

import (
	"fmt"
)

func main() {
	str := "a305a1748d8be5bf755073cf4d515be25ce23434d6b1f915432ba0d5f82f3dfe"
	a := []byte(str)
	fmt.Printf("s:%s, len:%d\n", str, len(str))
	fmt.Printf("a:%s, len:%d\n", a, len(a))
}
