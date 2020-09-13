/*
  This is free and unencumbered software released into the public domain. For more
  information, see <http://unlicense.org/> or the accompanying UNLICENSE file.
*/

package main

import (
	"fmt"
	"github.com/iand/imgpick"
	"os"
)

// A simple command line program for picking the best image from a web page
func main() {
	if len(os.Args) < 2 {
		println("Please supply url of webpage")
		os.Exit(1)
	}
	url := os.Args[1]

	res, err := imgpick.DetectMedia(url, true)

	if err != nil {
		fmt.Printf("Error reading from url: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Printf("Best image: %s\n", res.BestImage)
	fmt.Printf("Image urls\n")
	for _, img := range res.Images {
		fmt.Printf("* %s\n", img.Url)
	}

}
