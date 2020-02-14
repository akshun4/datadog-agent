// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-2020 Datadog, Inc.

// +build clusterchecks

package cloudfoundry

import (
	"code.cloudfoundry.org/bbs/models"
)

const (
	ENV_AD_VARIABLE_NAME = "AD_DATADOGHQ_COM"
	ENV_VCAP_SERVICES_VARIABLE_NAME = "VCAP_SERVICES"
)

type ActualLRP struct {
	AppGUID string
	CellID string
	ProcessGUID string
}

type DesiredLRP struct {
	AppGUID string
	EnvAD string
	EnvVcapServices string
	ProcessGUID string
}

func ActualLRPFromBBSModel(bbsLRP *models.ActualLRP) (ActualLRP) {
	a := ActualLRP{
		AppGUID:     appGUIDFromProcessGUID(bbsLRP.ProcessGuid),
		CellID:      bbsLRP.CellId,
		ProcessGUID: bbsLRP.ProcessGuid,
	}
	return a
}

func DesiredLRPFromBBSModel(bbsLRP *models.DesiredLRP) (DesiredLRP) {
	envAD, envVS := "", ""
	actionEnvs := [][]*models.EnvironmentVariable{}
	// Actions are a nested structure, e.g parallel action might contain two serial actions etc
	// We go through all actions breadth-first and record environment from all run actions,
	// since these are the ones we need to find
	actionQueue := []*models.Action{bbsLRP.Action}
	for len(actionQueue) > 0 {
		action := actionQueue[0]
		actionQueue = actionQueue[1:]

		if a := action.GetRunAction(); a != nil {
			actionEnvs = append(actionEnvs, a.Env)
		} else if a := action.GetTimeoutAction(); a != nil {
			actionQueue = append(actionQueue, a.Action)
		} else if a := action.GetEmitProgressAction(); a != nil {
			actionQueue = append(actionQueue, a.Action)
		} else if a := action.GetTryAction(); a != nil {
			actionQueue = append(actionQueue, a.Action)
		} else if a := action.GetParallelAction(); a != nil {
			actionQueue = append(actionQueue, a.Actions...)
		} else if a := action.GetSerialAction(); a != nil {
			actionQueue = append(actionQueue, a.Actions...)
		} else if a := action.GetCodependentAction(); a != nil {
			actionQueue = append(actionQueue, a.Actions...)
		}
	}

	for _, envVars := range actionEnvs {
		for _, ev := range envVars {
			if ev.Name == ENV_AD_VARIABLE_NAME {
				envAD = ev.Value
			} else if (ev.Name == ENV_VCAP_SERVICES_VARIABLE_NAME) {
				envVS = ev.Value
			}
		}
		if envAD != "" {
			// TODO: find out if there might be more different AD env variables in all actions (I think not)
			break
		}
	}
	d := DesiredLRP{
		AppGUID: appGUIDFromProcessGUID(bbsLRP.ProcessGuid),
		EnvAD: envAD,
		EnvVcapServices: envVS,
		ProcessGUID: bbsLRP.ProcessGuid,
	}
	return d
}

func appGUIDFromProcessGUID(processGUID string) string {
	return processGUID[0:36]
}
