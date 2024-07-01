package jobs

import (
	m "EagleEye/internal/models"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

var (
	hasPort      = regexp.MustCompile(`^https?:\/\/.*?:(\d{1,5})$`)
	hostWithPort = regexp.MustCompile(`^https?:\/\/(.*?:\d{1,5})$`)
)

func tempFileNSubsMap(subs []m.Subdomain) (string, map[string]*m.Subdomain, error) {
	tempFile, err := os.CreateTemp("/tmp/", "subs")
	if err != nil {
		return "", nil, fmt.Errorf("[!] Error creating temp file: %w", err)
	}

	subsMap := make(map[string]*m.Subdomain, len(subs))

	for _, sub := range subs {
		tempFile.WriteString(fmt.Sprintf("%s\n", sub.Subdomain))
		subsMap[sub.Subdomain] = &sub
	}

	tempFile.Close()

	return tempFile.Name(), subsMap, nil
}

func tempFileNServicesMap(services []m.HttpService) (string, map[string]*m.HttpService, error) {
	tempFile, err := os.CreateTemp("/tmp/", "services")
	if err != nil {
		return "", nil, fmt.Errorf("[!] Error creating temp file: %w", err)
	}

	servicesMap := make(map[string]*m.HttpService, len(services))

	for _, service := range services {
		tempFile.WriteString(fmt.Sprintf("%s\n", service.Host))
		servicesMap[service.Host] = &service
	}

	tempFile.Close()

	return tempFile.Name(), servicesMap, nil
}

func createEmptyHttps(httpSlice *[]interface{}, sub m.Subdomain) {
	PORTS := []int{80, 443}
	now := time.Now()

	for _, port := range PORTS {
		*httpSlice = append(*httpSlice,
			&m.HttpService{
				Subdomain: sub.ID,
				Host:      fmt.Sprintf("%s:%d", sub.Subdomain, port),
				IsActive:  false,
				Created:   nil,
				Updated:   now,
			},
		)
	}

}

func extractHostNUrl(host string) (string, string) {
	port := hasPort.FindString(host)
	if port != "" {
		return fmt.Sprintf("%s:%s", host, port), hostWithPort.FindString(host)
	}

	var url string
	var hostNport string

	if strings.HasPrefix(host, "http:") {
		url = fmt.Sprintf("%s:%d", host, 80)
		hostNport = fmt.Sprintf("%s:%d", host[7:], 80)
	} else {
		url = fmt.Sprintf("%s:%d", host, 443)
		hostNport = fmt.Sprintf("%s:%d", host[8:], 443)
	}

	return url, hostNport
}

func checkResults(results string) (string, error) {
	op := strings.TrimSpace(results)
	if op == "" {
		return "", ErrNoResult{}
	}

	return op, nil
}