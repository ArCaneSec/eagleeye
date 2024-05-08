package tools

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
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

func findAllParams(html string) (<-chan []string, <-chan error) {
	patterns := []string{`(?:<input.*?name)(?:="|')(.*?)(?:'|")`, // html name keys
		`(?:<input.*?id)(?:="|')(.*?)(?:'|")`, // html id keys
		`(?:(?:let|const|var)\s*)(\w+)`,       // JS variable names
		`(?:[{,]\s*(?:['"])?)(.+?)(?:\s*)(?=['"]?:)`,            // JS object keys
	}

	params := make(chan []string, len(patterns))
	errors := make(chan error, len(patterns))

	var wg sync.WaitGroup

	for _, pattern := range patterns {
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

func mergeParams(paramSlices [][]string, params chan string) {
	var wg sync.WaitGroup

	for _, slice := range paramSlices {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, val := range slice {
				params <- val
			}
		}()
	}
	wg.Wait()
	close(params)
}

func merge(params <-chan []string) <-chan string {

	var totalLen int
	var paramSlices [][]string

	// with knowing exact number of params, we can create a buffered channel.
	for slice := range params {
		totalLen += len(slice)
		paramSlices = append(paramSlices, slice)
	}

	allParams := make(chan string, totalLen)

	go func() {
		mergeParams(paramSlices, allParams)
	}()

	return allParams

}

func uniqueParams(params []string) ([]string, map[string]int) {
	seen := make(map[string]int, len(params))
	var uniqueParams []string

	for _, val := range params {
		if _, ok := seen[val]; !ok {
			seen[val] = 1
			uniqueParams = append(uniqueParams, val)
		} else {
			seen[val] += 1
		}
	}

	return uniqueParams, seen
}

func logError(errors <-chan error) (<-chan struct{}, error) {
	ex, _ := os.Executable()
	dir, _ := filepath.Split(filepath.Dir(ex))
	finalPath := filepath.Join(dir, "logs", "fallparams-logs.txt")

	file, err := os.OpenFile(finalPath, os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("[!] An error occurred while trying to open or create log file, error: %w", err)
	}

	done := make(chan struct{})
	go func() {
		for err := range errors {
			file.WriteString(err.Error() + "\n")
		}
		close(done)
	}()
	return done, nil
}

func FAllParams(url string) []string {
	htmlContent, _ := headlessRequest(url)
	
	params, errors := findAllParams(htmlContent)

	loggingDone, err := logError(errors)
	if err != nil {
		log.Fatal(err)
	}

	mergedParams := merge(params)
	var rs []string

	for val := range mergedParams {
		rs = append(rs, val)
	}
	uniques, _ := uniqueParams(rs)
	<-loggingDone
	fmt.Println("[*] Finished.")
	return uniques
}
