/*
  PUBLIC DOMAIN STATEMENT
  To the extent possible under law, Ian Davis has waived all copyright
  and related or neighboring rights to this Source Code file.
  This work is published from the United Kingdom.
*/

// Finds the primary image featured on a webpage
package imgpick

import (
	// "bytes"
	// "fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
)

func PickImage(pageUrl string) (image.Image, error) {
	re, err := regexp.Compile(`<img[^>]+src="([^"]+)"|<img[^>]+src='([^']+)'`)

	if err != nil {
		return nil, err
	}

	base, err := url.Parse(pageUrl)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(pageUrl)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var bestImage image.Image
	var bestArea int

	matches := re.FindAllSubmatch(content, -1)
	for _, match := range matches {
		srcValue := string(match[1])
		srcUrlRef, err := url.Parse(srcValue)
		if err != nil {
			continue
		}

		srcUrl := base.ResolveReference(srcUrlRef)

		imgResp, err := http.Get(srcUrl.String())
		if err != nil {
			continue
		}
		defer imgResp.Body.Close()
		img, _, err := image.Decode(imgResp.Body)
		if err != nil {
			continue
		}
		r := img.Bounds()
		area := (r.Max.X - r.Min.X) * (r.Max.Y - r.Min.Y)

		if area < 5000 {
			continue
		}

		sizeRatio := float64(r.Max.X-r.Min.X) / float64(r.Max.Y-r.Min.Y)
		if sizeRatio > 2 || sizeRatio < 0.5 {
			continue
		}

		if area > bestArea {
			bestArea = area
			bestImage = img
		}

	}

	if bestImage != nil {
		return bestImage, nil
	}
	return image.NewRGBA(image.Rect(0, 0, 50, 50)), nil
}
