package tools

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func headlessRequest(url string) (string, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.Flag("disable-web-security", true),
	)
	aCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(
		aCtx,
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
		"Referer":         url,
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
		return "", fmt.Errorf("[!] Error while running headless browser, check your url and network then try again.\nurl: %s\nerror: %w", url, err)
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
		`(?:<input.*?id)(?:="|')(.*?)(?:'|")`,             // html id keys
		`(?:(?:let|const|var)\s*)(\w+)`,                   // JS variable names
		`(?:[{,]\s*(?:['"])?)([\w_-]+?)(?:\s*)(?:['"]?:)`, // JS object keys
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
	go func() {
		wg.Wait()

		close(params)
		close(errors)
	}()
	return params, errors
}

func merge(params <-chan []string) <-chan string {
	mergedParams := make(chan string)
	var wg sync.WaitGroup

	for paramSlice := range params {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for _, val := range paramSlice {
				mergedParams <- val
			}
		}()
	}
	go func() {
		wg.Wait()
		close(mergedParams)
	}()

	return mergedParams
}

func uniqueParams(params <-chan string) ([]string, map[string]int) {
	seen := make(map[string]int, len(params))
	var uniqueParams []string

	for val := range params {
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

	file, err := os.OpenFile(finalPath, os.O_CREATE|os.O_APPEND, 0644)
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

func extractJsPath(html string) []string {
	r, err := regexp.Compile("(?:[\"'`])(.+\\.js(on)?)(?:[\"'`])")
	if err != nil {
		log.Fatal(err)
	}

	matches := r.FindAllStringSubmatch(html, -1)

	allPaths := make([]string, 0, len(matches))

	for _, path := range matches {
		if strings.HasPrefix(path[1], "http") || strings.HasPrefix(path[1], "//") {
			continue
		}

		allPaths = append(allPaths, path[1])
	}

	return allPaths
}

func sendRawRequest(host string, urls []string) (<-chan string, <-chan error) {
	client := http.Client{Timeout: 3 * time.Second}
	responses := make(chan string, len(urls))
	errors := make(chan error, len(urls))

	var wg sync.WaitGroup
	for _, url := range urls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fmt.Printf("[*] Sending request to %s\n", url)
			req, err := http.NewRequest("GET", host+"/"+url, nil)
			if err != nil {
				errors <- fmt.Errorf("[!] Failed to construct the request object, url: %s, err: %w", url, err)
				return
			}

			req.Header = http.Header{
				"User-Agent":      {"Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/114.0"},
				"Accept":          {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"},
				"Accept-Language": {"en-US,en;q=0.5"},
				"Sec-Fetch-Dest":  {"document"},
				"Sec-Fetch-Mode":  {"navigate"},
				"Sec-Fetch-Site":  {"none"},
				"Sec-Fetch-User":  {"?1"},
				"Referer":         {url},
			}

			res, err := client.Do(req)
			if err != nil {
				errors <- fmt.Errorf("[!] Request to %s url failed with error: %w", url, err)
				return
			}

			defer res.Body.Close()

			resBody, err := io.ReadAll(res.Body)
			if err != nil {
				fmt.Printf("[!] Failed to read response for %s url, error: %s\n", url, err.Error())
			}

			responses <- string(resBody)
		}()
	}
	go func() {
		wg.Wait()
		close(responses)
		close(errors)
	}()
	return responses, errors
}

func FAllParams(url string, crawl bool) []string {
	htmlContent, err := headlessRequest(url)
	if err != nil {
		log.Fatal(err)
	}

	if crawl {
		fmt.Println("[*] Crawling mode...")
		urls := extractJsPath(htmlContent)

		responses, _ := sendRawRequest(url, urls)

		var packedBodies []string
		packedBodies = append(packedBodies, htmlContent)

		for res := range responses {
			packedBodies = append(packedBodies, res)
		}

		htmlContent = strings.Join(packedBodies, ",")
	}

	params, _ := findAllParams(htmlContent)

	// loggingDone, err := logError(errors)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	mergedParams := merge(params)

	uniques, _ := uniqueParams(mergedParams)
	// <-loggingDone
	fmt.Println("[*] Finished.")

	return uniques
}
