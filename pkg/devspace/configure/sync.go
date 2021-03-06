package configure

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/covexo/devspace/pkg/devspace/config/configutil"
	"github.com/covexo/devspace/pkg/devspace/config/v1"
	"github.com/covexo/devspace/pkg/devspace/services"
)

// AddSyncPath adds a new sync path to the config
func AddSyncPath(localPath, containerPath, namespace, labelSelector, excludedPathsString, serviceName string) error {
	config := configutil.GetConfig()

	if config.DevSpace == nil {
		config.DevSpace = &v1.DevSpaceConfig{}
	}

	if config.DevSpace.Sync == nil {
		config.DevSpace.Sync = &[]*v1.SyncConfig{}
	}

	var labelSelectorMap map[string]*string
	var err error

	if labelSelector != "" && serviceName != "" {
		return fmt.Errorf("both service and label-selector specified. This is illegal because the label-selector is already specified in the referenced service. Therefore defining both is redundant")
	}

	if labelSelector == "" {
		config := configutil.GetConfig()

		if config.DevSpace != nil && config.DevSpace.Services != nil && len(*config.DevSpace.Services) > 0 {
			services := *config.DevSpace.Services

			var service *v1.ServiceConfig
			if serviceName != "" {
				service = getServiceWithName(*config.DevSpace.Services, serviceName)
				if service == nil {
					return fmt.Errorf("no service with name %v exists", serviceName)
				}
			} else {
				service = services[0]
			}
			labelSelectorMap = *service.LabelSelector
		} else {
			labelSelector = "release=" + services.GetNameOfFirstHelmDeployment()
		}
	}

	if labelSelectorMap == nil {
		labelSelectorMap, err = parseSelectors(labelSelector)
		if err != nil {
			return fmt.Errorf("Error parsing selectors: %s", err.Error())
		}
	}

	excludedPaths := make([]string, 0, 0)
	if excludedPathsString != "" {
		excludedPathStrings := strings.Split(excludedPathsString, ",")

		for _, v := range excludedPathStrings {
			excludedPath := strings.TrimSpace(v)
			excludedPaths = append(excludedPaths, excludedPath)
		}
	}

	workdir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Unable to determine current workdir: %s", err.Error())
	}

	localPath = strings.TrimPrefix(localPath, workdir)

	if containerPath[0] != '/' {
		return errors.New("ContainerPath (--container) must start with '/'. Info: There is an issue with MINGW based terminals like git bash")
	}

	//We set labelSelectorMap to nil since labelSelectorMap is already specified in service. Avoid redundance.
	if serviceName != "" {
		labelSelectorMap = nil
	}

	syncConfig := append(*config.DevSpace.Sync, &v1.SyncConfig{
		LabelSelector: &labelSelectorMap,
		ContainerPath: configutil.String(containerPath),
		LocalSubPath:  configutil.String(localPath),
		ExcludePaths:  &excludedPaths,
		Namespace:     &namespace,
		Service:       &serviceName,
	})

	config.DevSpace.Sync = &syncConfig

	err = configutil.SaveConfig()
	if err != nil {
		return fmt.Errorf("Couldn't save config file: %s", err.Error())
	}

	return nil
}

// RemoveSyncPath removes a sync path from the config
func RemoveSyncPath(removeAll bool, localPath, containerPath, labelSelector string) error {
	config := configutil.GetConfig()
	labelSelectorMap, err := parseSelectors(labelSelector)

	if err != nil {
		return fmt.Errorf("Error parsing selectors: %v", err)
	}

	if len(labelSelectorMap) == 0 && removeAll == false && localPath == "" && containerPath == "" {
		return fmt.Errorf("You have to specify at least one of the supported flags")
	}

	if config.DevSpace.Sync != nil && len(*config.DevSpace.Sync) > 0 {
		newSyncPaths := make([]*v1.SyncConfig, 0, len(*config.DevSpace.Sync)-1)

		for _, v := range *config.DevSpace.Sync {
			if removeAll ||
				localPath == *v.LocalSubPath ||
				containerPath == *v.ContainerPath ||
				areLabelMapsEqual(labelSelectorMap, *v.LabelSelector) {
				continue
			}

			newSyncPaths = append(newSyncPaths, v)
		}

		config.DevSpace.Sync = &newSyncPaths

		err = configutil.SaveConfig()
		if err != nil {
			return fmt.Errorf("Couldn't save config file: %v", err)
		}
	}

	return nil
}

func parseSelectors(selectorString string) (map[string]*string, error) {
	selectorMap := make(map[string]*string)

	if selectorString == "" {
		return selectorMap, nil
	}

	selectors := strings.Split(selectorString, ",")

	for _, v := range selectors {
		keyValue := strings.Split(v, "=")

		if len(keyValue) != 2 {
			return nil, fmt.Errorf("Wrong selector format: %s", selectorString)
		}
		labelSelector := strings.TrimSpace(keyValue[1])
		selectorMap[strings.TrimSpace(keyValue[0])] = &labelSelector
	}

	return selectorMap, nil
}

func areLabelMapsEqual(map1 map[string]*string, map2 map[string]*string) bool {
	if len(map1) != len(map2) {
		return false
	}

	for map1Index, map1Value := range map1 {
		if map2Value, map2Contains := map2[map1Index]; !map2Contains || *map2Value != *map1Value {
			return false
		}
	}

	return true
}
