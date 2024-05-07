package main

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func headlessRequest(url string) (string, error) {
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
		return "", fmt.Errorf("[!] Error while running headless browser: %w", err)
	}

	return htmlContent, nil
}

func extractParams(pattern string, html string, paramCh chan []string, errCh chan error) {
	r, err := regexp.Compile(pattern)

	if err != nil {
		errCh <- fmt.Errorf("[!] Error while compiling regex, regex: %s, error: %w", pattern, err)
		return
	}

	var params []string
	for _, match := range r.FindAllStringSubmatch(html, -1) {
		params = append(params, match[1])
	}

	paramCh <- params
}

func mergeParams(params chan []string, res chan []string) {
	var wg sync.WaitGroup
	var allparams []string

	for len(params) != 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allparams = append(allparams, <-params...)
		}()
	}
	wg.Wait()
	res <- allparams
}

func logErrors(errors <-chan error) {
	for len(errors) != 0 {
		fmt.Println(<-errors)
	}
}

func findAllParams(html string) (<-chan []string, <-chan error) {
	patterns := []string{`(?:<input.*?name)(?:="|')(.*?)(?:'|")`, // html name keys
		`(?:<input.*?id)(?:="|')(.*?)(?:'|")`, // html id keys
		`(?:(?:let|const|var)\s*)(\w+)`,       // JS variable names
		`(?:[{,]\s*(?:"|')?)(\w+)`,            // JS object keys
	}

	params := make(chan []string, len(patterns))
	errors := make(chan error, len(patterns))

	var wg sync.WaitGroup

	for _, pattern := range patterns {
		pattern := pattern
		wg.Add(1)
		go func() {
			defer wg.Done()
			extractParams(pattern, html, params, errors)
		}()
	}
	wg.Wait()

	close(params)
	close(errors)

	return params, errors

}

func merge(params <-chan []string, errors <-chan error) {

	var wg sync.WaitGroup
	wg.Add(2)
	chRes := make(chan string)
	go func() {
		defer wg.Done()
		for slc := range params {
			for _, val := range slc {
				chRes <- val
			}
		}
	}()
	go func() {
		defer wg.Done()
		logErrors(errors)
	}()
	wg.Wait()

}

func main() {
	// htmlContent := headlessRequest("http://localhost:3000/login")

	ss := `<form action="/action_page.php">
	<label for="fname">First name:</label>
	<input type="text" id="testid1" name="fname"><br><br>
	<label for="lname">Last name:</label>
	<input type="text" id="testid2" name="lname"><br><br>
	<input type="submit" value="Submit">
  </form>
 
  var obj = {
	"testkey1": "testval1",
	'testkey2': 'testval2',
	testkey3: testval3
  }

  let testlet = "somevalue"
  const testconst='someconst'
  var      testvar      =        "somevar"
  `

	merge(findAllParams(ss))
}
