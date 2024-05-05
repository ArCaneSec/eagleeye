package main

import (
	"context"
	"fmt"
	"log"
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

	matchedParams := r.FindAllStringSubmatch(html, -1)

	var params []string
	for _, param := range matchedParams {
		params = append(params, param[1])
	}

	ch <- params
}

func findHtmlNameParams(html string, ch chan []string) {
	const pattern = `(?:<input.*?name)(?:="|')(.*?)(?:'|")`

	extractParams(pattern, html, ch)
	wg.Done()
}

func findHtmlIdParams(html string, ch chan []string) {
	const pattern = `(?:<input.*?id)(?:="|')(.*?)(?:'|")`

	extractParams(pattern, html, ch)
	wg.Done()
}

var wg sync.WaitGroup

func main() {
	// htmlContent := headlessRequest("http://localhost:3000/login")

	ss := `<form action="/action_page.php">
	<label for="fname">First name:</label>
	<input type="text" id="testid1" name="fname"><br><br>
	<label for="lname">Last name:</label>
	<input type="text" id="testid2" name="lname"><br><br>
	<input type="submit" value="Submit">
  </form>`

	ch := make(chan []string, 2)

	wg.Add(2)
	go findHtmlNameParams(ss, ch)
	go findHtmlIdParams(ss, ch)
	wg.Wait()

	counter := 0
	for counter != 2 {
		for _, val := range <- ch {
			fmt.Println(val)
		}
		counter++
	}
}
