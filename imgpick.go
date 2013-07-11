/*
  This is free and unencumbered software released into the public domain. For more
  information, see <http://unlicense.org/> or the accompanying UNLICENSE file.
*/

// Finds the primary image featured on a webpage
package imgpick

import (
	"bytes"
	"cgl.tideland.biz/applog"
	"fmt"
	"github.com/iand/microdata"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ImageInfo struct {
	Url    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

type ImageData struct {
	ImageInfo
	Img image.Image
}

type DetectionResult struct {
	Title     string               `json:"title"`
	Url       string               `json:"url"`
	Images    []ImageInfo          `json:"images,omitempty"`
	MediaUrl  string               `json:"mediaurl"`
	MediaType string               `json:"mediatype"`
	Duration  int                  `json:"duration"`
	BestImage string               `json:"bestimage"`
	Microdata *microdata.Microdata `json:"microdata,omitempty"`
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

	readMicrodata(content, base, result)

	if result.Title == "" {
		result.Title = cleanTitle(firstMatch(content, titleRegexes))
	}

	if result.MediaUrl == "" {
		result.MediaUrl, result.MediaType = detectMedia(content, base)
	}

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

func readMicrodata(content []byte, base *url.URL, result *DetectionResult) {
	mdParser := microdata.NewParser(bytes.NewReader(content), base)

	md, _ := mdParser.Parse()
	// result.Microdata = md
	if len(md.Items) < 1 {
		return
	}

	for _, item := range md.Items {
		for _, t := range item.Types {
			switch t {
			case "http://schema.org/VideoObject":
				result.MediaType = "video"
				readMicrodataItem(item, result)
				return
			}
		}
	}

}

func readMicrodataItem(item *microdata.Item, result *DetectionResult) {

	result.Title = getMicrodataString(item, "name")
	result.MediaUrl = getMicrodataString(item, "url")
	result.Duration = parseIsoDuration(getMicrodataString(item, "duration"))

}

func getMicrodataString(item *microdata.Item, name string) string {
	if values, exists := item.Properties[name]; exists {
		for _, val := range values {
			switch typedValue := val.(type) {
			case string:
				return typedValue
			}
		}
	}

	return ""

}

func parseIsoDuration(duration string) int {
	println("DURATION: ", duration)
	val := 0

	if len(duration) < 3 {
		return 0
	}

	re, err := regexp.Compile(`([0-9]+)([A-Z])`)
	if err != nil {
		return val
	}

	matches := re.FindAllSubmatch([]byte(duration[2:]), -1)
	for _, match := range matches {
		valueStr := string(match[1])
		unit := string(match[2])
		println("FOUND: ", valueStr, unit)

		value, err := strconv.ParseInt(valueStr, 10, 0)
		if err != nil {
			return val
		}

		switch unit {
		case "S":
			val += int(value)
		case "M":
			val += int(value) * 60
		case "H":
			val += int(value) * 3600
		}

	}

	return val

}
