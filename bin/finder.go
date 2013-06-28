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

	mediaUrl, imageUrls, err := imgpick.FindMedia(url)

	if err != nil {
		fmt.Printf("Error reading from url: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Printf("Media url: %s\n", mediaUrl)
	fmt.Printf("Image urls\n")
	for _, i := range imageUrls {
		fmt.Printf("* %s\n", i)
	}

}
