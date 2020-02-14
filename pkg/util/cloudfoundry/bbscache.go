// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-2020 Datadog, Inc.

// +build clusterchecks

package cloudfoundry

import (
	"fmt"
	"sync"
	"time"

	"github.com/DataDog/datadog-agent/pkg/config"
	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

type BBSCache struct {
	// TODO: do we even need to lock this?
	sync.RWMutex
	configured bool
	bbsAPIClient bbs.Client
	bbsAPIClientLogger lager.Logger
	pollInterval time.Duration
	actualLRPs []ActualLRP
	desiredLRPs []DesiredLRP
}

var (
	globalBBSCache *BBSCache = &BBSCache{}
	globalBBSCacheLock sync.Mutex
)

func ConfigureGlobalBBSCache(conf config.ConfigurationProviders) (*BBSCache, error) {
	globalBBSCacheLock.Lock()
	defer globalBBSCacheLock.Unlock()
	var err error

	if globalBBSCache.configured {
		return globalBBSCache, nil
	}

	globalBBSCache.configured = true
	// TODO: how do we stop the cache (some channel?)
	// TODO: tests with insecure, prod code with secure
	clientConfig := bbs.ClientConfig{
		URL:                    conf.TemplateURL,
		IsTLS:                  false,
		CAFile:                 "", // conf.CAFile
		CertFile:               "", // conf.CertFile
		KeyFile:                "", // conf.KeyFile
		ClientSessionCacheSize: 0,
		MaxIdleConnsPerHost:    0,
		InsecureSkipVerify:     true,
		Retries:                10,
		RequestTimeout:         time.Duration(5 * time.Second),
	}
	globalBBSCache.bbsAPIClient, err = bbs.NewClientWithConfig(clientConfig)
	globalBBSCache.bbsAPIClientLogger = lager.NewLogger("bbs")
	// TODO: set poll interval from conf
	globalBBSCache.pollInterval = time.Duration(3 * time.Second)
	if err != nil {
		return nil, err
	}

	go globalBBSCache.start()

	return globalBBSCache, nil
}

func GetGlobalBBSCache() (*BBSCache, error) {
	if !globalBBSCache.configured {
		return nil, fmt.Errorf("Global BBS Cache not configured")
	}
	return globalBBSCache, nil
}

func (bc *BBSCache) GetActualLRPs() []ActualLRP {
	bc.RLock()
	defer bc.RUnlock()
	return bc.actualLRPs
}

func (bc *BBSCache) GetDesiredLRPs() []DesiredLRP {
	bc.RLock()
	defer bc.RUnlock()
	return bc.desiredLRPs
}

func (bc *BBSCache) start() {
	bc.readData()
	dataRefreshTicker := time.NewTicker(bc.pollInterval)
	for {
		select {
		case <- dataRefreshTicker.C:
			//fmt.Printf("%#v\n", bc.actualLRPs)
			//fmt.Printf("%#v\n", bc.desiredLRPs)
			bc.readData()
		}
	}
}

func (bc *BBSCache) readData() {
	// TODO: logging etc
	var wg sync.WaitGroup
	var actualLRPs []ActualLRP
	var desiredLRPs []DesiredLRP
	var errActual, errDesired error

	wg.Add(2)

	go func() {
		actualLRPs, errActual = bc.readActualLRPs()
		wg.Done()
	}()
	go func() {
		desiredLRPs, errDesired = bc.readDesiredLRPs()
		wg.Done()
	}()
	wg.Wait()
	if errActual != nil {
		return
	}
	if errDesired != nil {
		return
	}

	// put new values in cache
	bc.Lock()
	defer bc.Unlock()
	bc.actualLRPs = actualLRPs
	bc.desiredLRPs = desiredLRPs
}

func (bc *BBSCache) readActualLRPs() ([]ActualLRP, error) {
	// TODO: error handling - probably just log?
	actualLRPsBBS, err := bc.bbsAPIClient.ActualLRPs(bc.bbsAPIClientLogger, models.ActualLRPFilter{})
	if err != nil {
		return []ActualLRP{}, err
	}
	actualLRPs := make([]ActualLRP, len(actualLRPsBBS))
	for i, lrp := range actualLRPsBBS {
		actualLRPs[i] = ActualLRPFromBBSModel(lrp)
	}
	return actualLRPs, nil
}

func (bc *BBSCache) readDesiredLRPs() ([]DesiredLRP, error) {
	// TODO: error handling - probably just log?
	desiredLRPsBBS, err := bc.bbsAPIClient.DesiredLRPs(bc.bbsAPIClientLogger, models.DesiredLRPFilter{})
	if err != nil {
		return []DesiredLRP{}, err
	}
	desiredLRPs := make([]DesiredLRP, len(desiredLRPsBBS))
	for i, lrp := range desiredLRPsBBS {
		desiredLRPs[i] = DesiredLRPFromBBSModel(lrp)
	}
	return desiredLRPs, nil
}
