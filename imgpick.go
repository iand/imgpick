/*
  This is free and unencumbered software released into the public domain. For more
  information, see <http://unlicense.org/> or the accompanying UNLICENSE file.
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
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Url    string `json:"url"`
}

type ImageData struct {
	ImageInfo
	Img  image.Image
	Area int
}

var titleRegexes = []string{
	`<meta property="og:title" content="([^"]+)">`,
	`<meta property="twitter:title" content="([^"]+)">`,
	`<title>([^<]+)</title>`,
}

// Look for the image that best represents the given page and also
// a url for any embedded media
func PickImage(pageUrl string) (image.Image, error) {
	var currentBest ImageData

	_, _, imageUrls, err := FindMedia(pageUrl)
	if err != nil {
		return nil, err
	}

	currentBest, _ = selectBest(imageUrls, currentBest)

	if currentBest.Img != nil {
		return currentBest.Img, nil
	}

	return image.NewRGBA(image.Rect(0, 0, 50, 50)), nil
}

func FindMedia(pageUrl string) (mediaUrl string, title string, imageUrls []string, err error) {

	base, err := url.Parse(pageUrl)
	if err != nil {
		return "", "", imageUrls, err
	}

	resp, err := http.Get(pageUrl)
	if err != nil {
		return "", "", imageUrls, err
	}

	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", imageUrls, err
	}

	title = firstMatch(content, titleRegexes)

	seen := make(map[string]bool, 0)

	for _, url := range findYoutubeImages(content, base) {
		if _, exists := seen[url]; !exists {
			imageUrls = append(imageUrls, url)
			seen[url] = true
		}
	}

	for _, url := range findImageUrls(content, base) {
		if _, exists := seen[url]; !exists {
			imageUrls = append(imageUrls, url)
			seen[url] = true
		}
	}

	mediaUrl = detectMedia(content, base)

	return mediaUrl, title, imageUrls, err
}

func SelectBestImage(pageUrl string, imageUrls []string) (ImageData, []ImageInfo, error) {
	var currentBest ImageData
	var images []ImageInfo

	currentBest, images = selectBest(imageUrls, currentBest)

	if currentBest.Img != nil {
		return currentBest, images, nil
	}

	return ImageData{Img: image.NewRGBA(image.Rect(0, 0, 50, 50)), Area: 2500, ImageInfo: ImageInfo{Width: 50, Height: 50}}, images, nil
}

func resolveUrl(href string, base *url.URL) string {
	urlRef, err := url.Parse(href)
	if err != nil {
		return ""
	}

	srcUrl := base.ResolveReference(urlRef)
	return srcUrl.String()

}

func selectBest(urls []string, currentBest ImageData) (ImageData, []ImageInfo) {

	images := make([]ImageInfo, 0)

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

		images = append(images, ImageInfo{Url: url, Width: (r.Max.X - r.Min.X), Height: (r.Max.Y - r.Min.Y)})

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

	return currentBest, images

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

	re1, err := regexp.Compile(`//www.youtube.com/watch\?v=([A-Za-z0-9-]+)`)
	if err != nil {
		return urls
	}

	re2, err := regexp.Compile(`//www.youtube.com/embed/([A-Za-z0-9-]+)`)
	if err != nil {
		return urls
	}

	matches := re1.FindAllSubmatch(content, -1)
	for _, match := range matches {
		key := string(match[1])

		url := fmt.Sprintf("https://img.youtube.com/vi/%s/0.jpg", key)

		urls = append(urls, url)
	}

	matches = re2.FindAllSubmatch(content, -1)
	for _, match := range matches {
		key := string(match[1])

		url := fmt.Sprintf("https://img.youtube.com/vi/%s/0.jpg", key)

		urls = append(urls, url)
	}

	return urls

}

func detectMedia(content []byte, base *url.URL) string {

	switch {
	case base.Host == "youtube.com" || base.Host == "www.youtube.com":
		re, err := regexp.Compile(`<meta property="og:url" content="([^"]+)">`)
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

func firstMatch(content []byte, regexes []string) string {

	for _, r := range regexes {
		re, err := regexp.Compile(r)
		if err != nil {
			continue
		}

		matches := re.FindAllSubmatch(content, -1)
		if len(matches) > 0 {
			return string(matches[0][1])
		}

	}

	return ""

}
