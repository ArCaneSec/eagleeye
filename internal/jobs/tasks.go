package jobs

import (
	m "EagleEye/internal/models"
	"EagleEye/internal/notifs"
	"context"
	"sync"

	"go.mongodb.org/mongo-driver/mongo"
)

type Task interface {
	run(context.Context)
	fetchAssets(context.Context) error
}

type Dependencies struct {
	db     *mongo.Database
	notify notifs.Notify
	wg     *sync.WaitGroup
}

type SubdomainEnumeration struct {
	*Dependencies
	scriptPath string
	targets    []m.Target
}

type DnsResolve struct {
	*Dependencies
	scriptPath string
	subdomains []m.Subdomain
}

type DnsResolveAll struct {
	*DnsResolve
}

type HttpDiscovery struct {
	*Dependencies
	scriptPath string
	hosts      []m.HttpService
}

type HttpDiscoveryAll struct {
	*HttpDiscovery
}
