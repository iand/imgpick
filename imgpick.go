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
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
)

type ImageInfo struct {
	Img  image.Image
	Area int
	Url  string
}

// Look for the image that best represents the given page and also
// a url for any embedded media
func PickImage(pageUrl string) (image.Image, string, string, error) {
	var currentBest ImageInfo

	base, err := url.Parse(pageUrl)
	if err != nil {
		return nil, "", "", err
	}

	resp, err := http.Get(pageUrl)
	if err != nil {
		return nil, "", "", err
	}

	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", err
	}

	println("got %d", len(content))
	currentBest = selectBest(findImageUrls(content, base), currentBest)
	currentBest = selectBest(findYoutubeImages(content, base), currentBest)
	mediaUrl := detectMedia(content, base)

	if currentBest.Img != nil {
		return currentBest.Img, currentBest.Url, mediaUrl, nil
	}

	return image.NewRGBA(image.Rect(0, 0, 50, 50)), "", mediaUrl, nil
}

func resolveUrl(href string, base *url.URL) string {
	urlRef, err := url.Parse(href)
	if err != nil {
		return ""
	}

	srcUrl := base.ResolveReference(urlRef)
	return srcUrl.String()

}

func selectBest(urls []string, currentBest ImageInfo) ImageInfo {

	for _, url := range urls {

		imgResp, err := http.Get(url)
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

		if area > currentBest.Area {
			currentBest.Area = area
			currentBest.Img = img
			currentBest.Url = url
		}

	}

	return currentBest

}

func findImageUrls(content []byte, base *url.URL) []string {
	var urls []string

	re, err := regexp.Compile(`<img[^>]+src="([^"]+)"|<img[^>]+src='([^']+)'`)
	if err != nil {
		return urls
	}

	matches := re.FindAllSubmatch(content, -1)
	for _, match := range matches {
		srcUrl := resolveUrl(string(match[1]), base)
		urls = append(urls, srcUrl)
	}

	return urls

}

func findYoutubeImages(content []byte, base *url.URL) []string {
	var urls []string

	re, err := regexp.Compile(`http://www.youtube.com/watch\?v=([A-Za-z0-9]+)`)
	if err != nil {
		return urls
	}

	matches := re.FindAllSubmatch(content, -1)
	for _, match := range matches {
		key := string(match[1])

		url := fmt.Sprintf("https://img.youtube.com/vi/%s/0.jpg", key)

		urls = append(urls, url)
	}

	return urls

}

func detectMedia(content []byte, base *url.URL) string {

	switch {
	case base.Host == "youtube.com:80" || base.Host == "www.youtube.com:80":
		re, err := regexp.Compile(`<meta property="og:url" content="([^"]+">`)
		if err != nil {
			return ""
		}

		matches := re.FindAllSubmatch(content, -1)
		if len(matches) > 0 {
			return string(matches[0][1])
		}

	}

	return ""
}
