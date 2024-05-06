package main

import (
	"context"
	"log"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func headlessRequest(url string) string {
	ctx, cancel := chromedp.NewContext(
		context.Background(),
	)

	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	defaultHeaders := map[string]interface{}{
		"User-Agent":      "Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/114.0",
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		"Accept-Language": "en-US,en;q=0.5",
		"Sec-Fetch-Dest":  "document",
		"Sec-Fetch-Mode":  "navigate",
		"Sec-Fetch-Site":  "none",
		"Sec-Fetch-User":  "?1",
		"Referer":         "test.com",
	}

	var htmlContent string
	err := chromedp.Run(ctx,
		network.Enable(),
		network.SetExtraHTTPHeaders(defaultHeaders),
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.OuterHTML("html", &htmlContent),
	)
	if err != nil {
		log.Fatal(err)
	}

	return htmlContent
}

func extractParams(pattern string, html string, ch chan []string) {
	r, err := regexp.Compile(pattern)

	if err != nil {
		log.Fatal(err)
	}

	var params []string
	for _, match := range r.FindAllStringSubmatch(html, -1) {
		params = append(params, match[1])
	}

	ch <- params
}

func findHtmlNameParams(html string, ch chan []string) {
	const pattern = `(?:<input.*?name)(?:="|')(.*?)(?:'|")`

	go func() {
		extractParams(pattern, html, ch)
	}()
}

func findHtmlIdParams(html string, ch chan []string) {
	const pattern = `(?:<input.*?id)(?:="|')(.*?)(?:'|")`
	go func() {
		extractParams(pattern, html, ch)
	}()

}

func main() {
	// htmlContent := headlessRequest("http://localhost:3000/login")

	ss := `<form action="/action_page.php">
	<label for="fname">First name:</label>
	<input type="text" id="testid1" name="fname"><br><br>
	<label for="lname">Last name:</label>
	<input type="text" id="testid2" name="lname"><br><br>
	<input type="submit" value="Submit">
  </form>`

	params := make(chan []string, 2)

	findHtmlNameParams(ss, params)
	findHtmlIdParams(ss, params)

	var wg sync.WaitGroup
	var results []string

	for i := 0; i != 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results = append(results, <-params...)
		}()
	}
	wg.Wait()
	file, err := os.OpenFile("test.txt", os.O_CREATE, 0664)

	if err != nil {
		log.Fatal(err)
	}
	for _, param := range results {
		file.WriteString(param+"\n")
	}
}
