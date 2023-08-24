package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Printf("Starting give-me-dns...")
	Init(os.Args[1])
}
