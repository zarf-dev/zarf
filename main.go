//go:generate fileb0x assets.yaml
package main

import (
	"log"
	"net/http"

	// "example.com/foo/simple" represents your package (as per go.mod)
	// package assets is created by `go generate` according to b0x.yaml (as per the comment above)
	"shift/assets"
)

func main() {
	files, err := assets.WalkDirs("", false)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("ALL FILES", files)

	b, err := assets.ReadFile("README.md")
	if err != nil {
		log.Fatal(err)
	}

	_ = b
	//log.Println(string(b))
	log.Println("try it -> http://localhost:8080/secrets.txt")

	// false = file system
	// true = handler
	as := false

	// try it -> http://localhost:8080/secrets.txt
	if as {
		// as Handler
		panic(http.ListenAndServe(":8080", assets.Handler))
	} else {
		// as File System
		panic(http.ListenAndServe(":8080", http.FileServer(assets.HTTP)))
	}
}