/*
  This is free and unencumbered software released into the public domain. For more
  information, see <http://unlicense.org/> or the accompanying UNLICENSE file.
*/

// Finds the primary image featured on a webpage
package imgpick

import (
	"cgl.tideland.biz/applog"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type ImageInfo struct {
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	Url    string `json:"url"`
}

type ImageData struct {
	ImageInfo
	Img image.Image
}

type DetectionResult struct {
	Title     string      `json:"title"`
	Url       string      `json:"url"`
	Images    []ImageInfo `json:"images,omitempty"`
	MediaUrl  string      `json:"mediaurl"`
	MediaType string      `json:"mediatype"`
	BestImage string      `json:"bestimage"`
}

var titleRegexes = []string{
	`<meta property="og:title" content="([^"]+)">`,
	`<meta property="twitter:title" content="([^"]+)">`,
	`<title>([^<]+)</title>`,
}

// Detects the primary subject of the given URL and returns metadata about it
func DetectMedia(url string, selectBest bool) (*DetectionResult, error) {
	result, err := ExtractMedia(url)

	//mediaUrl, title, imageUrls, err := FindMedia(url)

	if err != nil {
		return nil, err
	}

	if selectBest {
		fetchImageDimensions(result)
		selectBestImage(result)

	}
	return result, nil

}

// // Look for the image that best represents the given page and also
// // a url for any embedded media
// func PickImage(pageUrl string) (image.Image, error) {
// 	var currentBest ImageData

// 	_, _, imageUrls, err := FindMedia(pageUrl)
// 	if err != nil {
// 		return nil, err
// 	}

// 	currentBest, _ = selectBest(imageUrls, currentBest)

// 	if currentBest.Img != nil {
// 		return currentBest.Img, nil
// 	}

// 	return image.NewRGBA(image.Rect(0, 0, 50, 50)), nil
// }

// Extract the primary subject of the given URL and returns metadata about it
// without fetching any more URLs
func ExtractMedia(pageUrl string) (*DetectionResult, error) {
	result := &DetectionResult{
		Url: pageUrl,
	}
	base, err := url.Parse(pageUrl)
	if err != nil {
		return result, err
	}

	resp, err := http.Get(pageUrl)
	if err != nil {
		return result, err
	}

	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}

	result.Title = cleanTitle(firstMatch(content, titleRegexes))

	result.MediaUrl, result.MediaType = detectMedia(content, base)

	seen := make(map[string]bool, 0)
	for _, url := range findYoutubeImages(content, base) {
		if _, exists := seen[url]; !exists {
			result.Images = append(result.Images, ImageInfo{Url: url})
			seen[url] = true
		}
	}

	for _, url := range findImageUrls(content, base) {
		if _, exists := seen[url]; !exists {
			result.Images = append(result.Images, ImageInfo{Url: url})
			seen[url] = true
		}
	}

	return result, err
}

func selectBestImage(result *DetectionResult) {
	var currentBest ImageInfo

	for _, imginfo := range result.Images {
		sizeRatio := float64(imginfo.Width) / float64(imginfo.Height)
		if sizeRatio > 2 || sizeRatio < 0.5 {
			continue
		}

		area := imginfo.Width * imginfo.Height
		if area < 5000 {
			continue
		}

		if area > (currentBest.Width * currentBest.Height) {
			currentBest = imginfo
		}

	}

	result.BestImage = currentBest.Url

}

func resolveUrl(href string, base *url.URL) string {
	urlRef, err := url.Parse(href)
	if err != nil {
		return ""
	}

	srcUrl := base.ResolveReference(urlRef)
	return srcUrl.String()

}

func fetchImageDimensions(result *DetectionResult) {

	images := make([]ImageInfo, 0)
	urlchan := make(chan string, len(result.Images))
	results := make(chan *ImageInfo, 0)
	quit := make(chan bool, 0)

	go fetchImage(urlchan, results, quit)
	go fetchImage(urlchan, results, quit)
	go fetchImage(urlchan, results, quit)
	go fetchImage(urlchan, results, quit)

	for _, imageinfo := range result.Images {
		urlchan <- imageinfo.Url
	}

	timeout := time.After(time.Duration(500) * time.Millisecond)
	for i := 0; i < len(result.Images); i++ {
		select {
		case result := <-results:
			if result != nil {
				images = append(images, *result)
			}
		case <-timeout:
			applog.Debugf("Search timed out")
			close(quit)
			result.Images = images
			return
		}
	}

	close(quit)
	result.Images = images
	return

}

func fetchImage(urls chan string, results chan *ImageInfo, quit chan bool) {

	for {
		select {
		case url := <-urls:
			applog.Debugf("Fetching image %s", url)
			imgResp, err := http.Get(url)
			if err != nil {
				applog.Errorf("Error fetching image from %s: %s", url, err.Error())
				results <- nil
				continue
			}
			defer imgResp.Body.Close()
			img, _, err := image.Decode(imgResp.Body)
			if err != nil {
				applog.Errorf("Error decoding image from %s: %s", url, err.Error())
				results <- nil
				continue
			}
			r := img.Bounds()

			results <- &ImageInfo{
				Url:    url,
				Width:  (r.Max.X - r.Min.X),
				Height: (r.Max.Y - r.Min.Y),
			}

		case <-quit:
			applog.Debugf("Image fetcher quitting")
			return
		}
	}
}

func findImageUrls(content []byte, base *url.URL) []string {

	relist := []string{
		`<img[^>]+src="([^"]+)"`,
		`<img[^>]+src='([^']+)'`,
	}

	var urls []string

	for _, match := range allMatches(content, relist) {
		srcUrl := resolveUrl(match, base)
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

func detectMedia(content []byte, base *url.URL) (string, string) {

	switch {
	case base.Host == "youtube.com" || base.Host == "www.youtube.com":
		re, err := regexp.Compile(`<meta property="og:url" content="([^"]+)">`)
		if err != nil {
			return "", ""
		}

		matches := re.FindAllSubmatch(content, -1)
		if len(matches) > 0 {
			return string(matches[0][1]), "video"
		}

	}

	return "", ""
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

func allMatches(content []byte, regexes []string) []string {
	results := make([]string, 0)

	for _, r := range regexes {
		re, err := regexp.Compile(r)
		if err != nil {
			continue
		}

		matches := re.FindAllSubmatch(content, -1)
		for _, match := range matches {
			results = append(results, string(match[1]))
		}

	}

	return results

}

func cleanTitle(title string) string {
	if pos := strings.Index(title, " |"); pos != -1 {
		title = title[:pos]
	}

	if pos := strings.Index(title, " —"); pos != -1 {
		title = title[:pos]
	}

	if pos := strings.Index(title, " - "); pos != -1 {
		title = title[:pos]
	}

	if pos := strings.Index(title, "&nbsp;-&nbsp;"); pos != -1 {
		title = title[:pos]
	}

	title = strings.Trim(title, " ")
	return title
}
