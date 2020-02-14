// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-2020 Datadog, Inc.
//
// +build clusterchecks

package providers

import (
	"encoding/json"
	"fmt"
	"github.com/DataDog/datadog-agent/pkg/autodiscovery/integration"
	"github.com/DataDog/datadog-agent/pkg/autodiscovery/providers/names"
	"github.com/DataDog/datadog-agent/pkg/config"
	"github.com/DataDog/datadog-agent/pkg/util/cloudfoundry"
	"github.com/DataDog/datadog-agent/pkg/util/log"
)

type CloudFoundryConfigProvider struct {
	bbsCache *cloudfoundry.BBSCache
}

func NewCloudFoundryConfigProvider(conf config.ConfigurationProviders) (ConfigProvider, error) {
	cfp := CloudFoundryConfigProvider{}
	var err error

	cfp.bbsCache, err = cloudfoundry.ConfigureGlobalBBSCache(conf)
	if err != nil {
		return nil, err
	}
	return cfp, nil
}

func (cf CloudFoundryConfigProvider) String() string {
	return names.CloudFoundryBBS
}

func (cf CloudFoundryConfigProvider) IsUpToDate() (bool, error) {
	// TODO
	return false, nil
}

// Collect collects AD config templates from all Desired LRPs from BBS API
func (cf CloudFoundryConfigProvider) Collect() ([]integration.Config, error) {
	desiredLRPs := cf.bbsCache.GetDesiredLRPs()
	allConfigs := []integration.Config{}
	for _, desiredLRP := range desiredLRPs {
		newConfigs, err := cf.getConfigsFromDesiredLRP(desiredLRP)
		if err != nil {
			// TODO: logging
			fmt.Println(err.Error())
		}
		allConfigs = append(allConfigs, newConfigs...)
	}
	fmt.Printf("%#v\n", allConfigs)
	return allConfigs, nil
}

func (cf CloudFoundryConfigProvider) getConfigsFromDesiredLRP(desiredLRP cloudfoundry.DesiredLRP) ([]integration.Config, error) {
	allConfigs := []integration.Config{}
	// if EnvAD is not set, there are no configs to generate
	if desiredLRP.EnvAD == "" {
		return allConfigs, nil
	}

	// the AD config looks like:
	// {"my-http-app": {"check_names": ..., "init_configs": ..., "instances": ...}, ...}
	// we need to unmarshal the values of check_names, init_configs and instances to json.RawMessage
	// to be able to pass them to extractTemplatesFromMap
	var adConfig map[string]map[string]json.RawMessage
	err := json.Unmarshal([]byte(desiredLRP.EnvAD), &adConfig)
	if err != nil {
		return allConfigs, err
	}

	for adName, adVal := range adConfig {
		// for every AD top-level key (my-http-app in example above), we may create multiple configs
		id := fmt.Sprintf("%s/%s", desiredLRP.ProcessGUID, adName)
		// we need to convert adVal to map[string]string to pass it to extractTemplatesFromMap
		convertedADVal := map[string]string{}
		for k, v := range adVal {
			convertedADVal[k] = string(v)
		}
		parsedConfigs, errs := extractTemplatesFromMap(id, convertedADVal, "")
		for _, err := range errs {
			log.Errorf("Cannot parse endpoint template for service %s/%s: %s", desiredLRP.ProcessGUID, adName, err)
		}
		allConfigs = append(allConfigs, parsedConfigs...)
	}

	return allConfigs, nil
}

func init() {
	RegisterProvider(names.CloudFoundryBBS, NewCloudFoundryConfigProvider)
}
