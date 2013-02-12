/*
  PUBLIC DOMAIN STATEMENT
  To the extent possible under law, Ian Davis has waived all copyright
  and related or neighboring rights to this Source Code file.
  This work is published from the United Kingdom.
*/

package main

import (
	imgpick ".."
	"fmt"
	"image/png"
	"os"
)

// A simple command line program for picking the best image from a web page
func main() {
	if len(os.Args) < 3 {
		println("Please supply url of webpage and filename to write image to")
		os.Exit(1)
	}
	url := os.Args[1]
	foutName := os.Args[2]

	fout, err := os.OpenFile(foutName, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		fmt.Printf("Error writing output image: %s\n", err.Error())
		os.Exit(1)
	}

	img, err := imgpick.PickImage(url)

	if err != nil {
		fmt.Printf("Error reading from url: %s\n", err.Error())
		os.Exit(1)
	}

	if err = png.Encode(fout, img); err != nil {
		fmt.Printf("Error encoding output image: %s\n", err.Error())
		os.Exit(1)
	}

}