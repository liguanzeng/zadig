/*
Copyright 2024 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package job

import (
	"fmt"

	sae "github.com/alibabacloud-go/sae-20190506/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	saeservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/sae"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"k8s.io/apimachinery/pkg/util/sets"
)

type SAEReleaseJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.SAEReleaseJobSpec
}

func (j *SAEReleaseJob) Instantiate() error {
	j.spec = &commonmodels.SAEReleaseJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *SAEReleaseJob) SetPreset() error {
	j.spec = &commonmodels.SAEReleaseJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}

	// create env options for frontend to select
	envOptions, err := generateSAEEnvOption(j.workflow.Project)
	if err != nil {
		return err
	}
	j.spec.EnvOptions = envOptions

	// fill in the defaulted selected app info for frontend
	selectedServiceList, err := generateSAEDefaultSelectedService(j.workflow.Project, j.spec.EnvConfig.Name, j.spec.ServiceConfig.DefaultServices)
	if err != nil {
		return err
	}
	j.spec.ServiceConfig.Services = selectedServiceList

	j.job.Spec = j.spec
	return nil
}

func (j *SAEReleaseJob) ClearSelectionField() error {
	j.spec = &commonmodels.SAEReleaseJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}

	j.spec.EnvConfig.Name = ""
	j.spec.ServiceConfig.Services = make([]*commonmodels.SAEDeployServiceInfo, 0)

	j.job.Spec = j.spec
	return nil
}

func (j *SAEReleaseJob) SetOptions() error {
	j.spec = &commonmodels.SAEReleaseJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}

	// create env options for frontend to select
	envOptions, err := generateSAEEnvOption(j.workflow.Project)
	if err != nil {
		return err
	}
	j.spec.EnvOptions = envOptions

	j.job.Spec = j.spec
	return nil
}

func (j *SAEReleaseJob) ClearOptions() error {
	j.spec = &commonmodels.SAEReleaseJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}

	j.spec.EnvOptions = make([]*commonmodels.SAEEnvInfo, 0)

	j.job.Spec = j.spec
	return nil
}

func (j *SAEReleaseJob) MergeArgs(args *commonmodels.Job) error {
	j.spec = &commonmodels.SAEReleaseJobSpec{}
	if err := commonmodels.IToi(args.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *SAEReleaseJob) UpdateWithLatestSetting() error {
	j.spec = &commonmodels.SAEReleaseJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}

	latestWorkflow, err := commonrepo.NewWorkflowV4Coll().Find(j.workflow.Name)
	if err != nil {
		log.Errorf("Failed to find original workflow to set options, error: %s", err)
	}

	latestSpec := new(commonmodels.SAEReleaseJobSpec)
	found := false
	for _, stage := range latestWorkflow.Stages {
		if !found {
			for _, job := range stage.Jobs {
				if job.Name == j.job.Name && job.JobType == j.job.JobType {
					if err := commonmodels.IToi(job.Spec, latestSpec); err != nil {
						return err
					}
					found = true
					break
				}
			}
		} else {
			break
		}
	}

	if !found {
		return fmt.Errorf("failed to find the original workflow: %s", j.workflow.Name)
	}

	j.job.Spec = j.spec
	return nil
}

func (j *SAEReleaseJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	j.spec = &commonmodels.SAEReleaseJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return nil, err
	}

	if j.spec.ServiceConfig.Source == config.SourceFromJob {
		if j.spec.OriginJobName != "" {
			j.spec.JobName = j.spec.OriginJobName
		}

	}

	j.job.Spec = j.spec

	return nil, nil
}

func generateSAEEnvOption(projectKey string) (envOptions []*commonmodels.SAEEnvInfo, err error) {
	saeModel, err := commonrepo.NewSAEColl().FindDefault()
	if err != nil {
		err = fmt.Errorf("failed to find default sae, err: %s", err)
		log.Error(err)
		return nil, err
	}

	envOptions = make([]*commonmodels.SAEEnvInfo, 0)

	envs, err := commonrepo.NewSAEEnvColl().List(&commonrepo.SAEEnvListOptions{
		ProjectName: projectKey,
	})
	if err != nil {
		log.Errorf("failed to list sae envs for project: %s, error: %s", projectKey, err)
		return nil, fmt.Errorf("failed to list sae envs for project: %s, error: %s", projectKey, err)
	}

	for _, env := range envs {
		serviceList := make([]*commonmodels.SAEServiceInfo, 0)

		saeClient, err := saeservice.NewClient(saeModel, env.RegionID)
		if err != nil {
			err = fmt.Errorf("failed to create sae client, err: %s", err)
			log.Error(err)
			return nil, err
		}

		tags := fmt.Sprintf(`[{"Key":"%s","Value":"%s"}, {"Key":"%s","Value":"%s"}]`, setting.SAEZadigProjectTagKey, projectKey, setting.SAEZadigEnvTagKey, env.EnvName)

		saeRequest := &sae.ListApplicationsRequest{
			Tags:        tea.String(tags),
			CurrentPage: tea.Int32(1),
			// TODO: possibly fix the hard-coded paging.
			PageSize: tea.Int32(10000),
		}

		saeResp, err := saeClient.ListApplications(saeRequest)
		if err != nil {
			err = fmt.Errorf("failed to list applications, err: %s", err)
			log.Error(err)
			return nil, err
		}

		if !tea.BoolValue(saeResp.Body.Success) {
			err = fmt.Errorf("failed to list applications, statusCode: %d, code: %s, errCode: %s, message: %s", tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
			log.Error(err)
			return nil, err
		}

		for _, saeApp := range saeResp.Body.Data.Applications {
			// if the sae app has not been tagged with the service name and service module, we ignore it
			tagged := false
			serviceName := ""
			serviceModule := ""
			for _, tag := range saeApp.Tags {
				if tea.StringValue(tag.Key) == setting.SAEZadigServiceTagKey {
					tagged = true
					serviceName = tea.StringValue(tag.Value)
				}

				if tea.StringValue(tag.Key) == setting.SAEZadigServiceModuleTagKey {
					serviceModule = tea.StringValue(tag.Value)
				}

			}

			if !tagged {
				continue
			}

			describeAppReq := &sae.DescribeApplicationConfigRequest{
				AppId: saeApp.AppId,
			}

			appDetailResp, err := saeClient.DescribeApplicationConfig(describeAppReq)
			if err != nil {
				err = fmt.Errorf("failed to list applications, err: %s", err)
				log.Error(err)
				return nil, err
			}

			if !tea.BoolValue(appDetailResp.Body.Success) {
				err = fmt.Errorf("failed to describe application, statusCode: %d, code: %s, errCode: %s, message: %s", tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
				log.Error(err)
				return nil, err
			}

			kv := make([]*commonmodels.SAEKV, 0)

			saeKVMap, err := saeservice.CreateKVMap(appDetailResp.Body.Data.Envs)
			if err != nil {
				err = fmt.Errorf("failed to decode sae app's env variables, error: %s", err)
				log.Error(err)
				return nil, err
			}

			for _, saeKV := range saeKVMap {
				kv = append(kv, saeKV)
			}

			serviceList = append(serviceList, &commonmodels.SAEServiceInfo{
				AppID:         tea.StringValue(saeApp.AppId),
				AppName:       tea.StringValue(saeApp.AppName),
				Image:         tea.StringValue(saeApp.ImageUrl),
				Instances:     tea.Int32Value(saeApp.Instances),
				Envs:          kv,
				ServiceName:   serviceName,
				ServiceModule: serviceModule,
			})
		}

		envOptions = append(envOptions, &commonmodels.SAEEnvInfo{
			Env:      env.EnvName,
			Services: serviceList,
		})
	}

	return envOptions, nil
}

func generateSAEDefaultSelectedService(projectKey, envName string, defaultServices []*commonmodels.ServiceNameAndModule) (selectedServiceList []*commonmodels.SAEDeployServiceInfo, err error) {
	saeModel, err := commonrepo.NewSAEColl().FindDefault()
	if err != nil {
		err = fmt.Errorf("failed to find default sae, err: %s", err)
		log.Error(err)
		return nil, err
	}

	envs, err := commonrepo.NewSAEEnvColl().List(&commonrepo.SAEEnvListOptions{
		ProjectName: projectKey,
	})
	if err != nil {
		log.Errorf("failed to list sae envs for project: %s, error: %s", projectKey, err)
		return nil, fmt.Errorf("failed to list sae envs for project: %s, error: %s", projectKey, err)
	}

	// selectedServiceList will be filled with user-configured default service with the app's information in the env
	selectedServiceList = make([]*commonmodels.SAEDeployServiceInfo, 0)

	for _, env := range envs {
		saeClient, err := saeservice.NewClient(saeModel, env.RegionID)
		if err != nil {
			err = fmt.Errorf("failed to create sae client, err: %s", err)
			log.Error(err)
			return nil, err
		}

		tags := fmt.Sprintf(`[{"Key":"%s","Value":"%s"}, {"Key":"%s","Value":"%s"}]`, setting.SAEZadigProjectTagKey, projectKey, setting.SAEZadigEnvTagKey, env.EnvName)

		saeRequest := &sae.ListApplicationsRequest{
			Tags:        tea.String(tags),
			CurrentPage: tea.Int32(1),
			// TODO: possibly fix the hard-coded paging.
			PageSize: tea.Int32(10000),
		}

		saeResp, err := saeClient.ListApplications(saeRequest)
		if err != nil {
			err = fmt.Errorf("failed to list applications, err: %s", err)
			log.Error(err)
			return nil, err
		}

		if !tea.BoolValue(saeResp.Body.Success) {
			err = fmt.Errorf("failed to list applications, statusCode: %d, code: %s, errCode: %s, message: %s", tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
			log.Error(err)
			return nil, err
		}

		isSelectedEnv := env.EnvName == envName

		// if this env is the default selected env, we throw the info into the
		defaultServiceMap := sets.NewString()
		if !isSelectedEnv {
			continue
		}

		for _, service := range defaultServices {
			key := fmt.Sprintf("%s++%s", service.ServiceName, service.ServiceModule)
			defaultServiceMap.Insert(key)
		}

		for _, saeApp := range saeResp.Body.Data.Applications {
			// if the sae app has not been tagged with the service name and service module, we ignore it
			tagged := false
			serviceName := ""
			serviceModule := ""
			for _, tag := range saeApp.Tags {
				if tea.StringValue(tag.Key) == setting.SAEZadigServiceTagKey {
					tagged = true
					serviceName = tea.StringValue(tag.Value)
				}

				if tea.StringValue(tag.Key) == setting.SAEZadigServiceModuleTagKey {
					serviceModule = tea.StringValue(tag.Value)
				}

			}

			if !tagged {
				continue
			}

			describeAppReq := &sae.DescribeApplicationConfigRequest{
				AppId: saeApp.AppId,
			}

			appDetailResp, err := saeClient.DescribeApplicationConfig(describeAppReq)
			if err != nil {
				err = fmt.Errorf("failed to list applications, err: %s", err)
				log.Error(err)
				return nil, err
			}

			if !tea.BoolValue(appDetailResp.Body.Success) {
				err = fmt.Errorf("failed to describe application, statusCode: %d, code: %s, errCode: %s, message: %s", tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
				log.Error(err)
				return nil, err
			}

			kv := make([]*commonmodels.SAEKV, 0)

			saeKVMap, err := saeservice.CreateKVMap(appDetailResp.Body.Data.Envs)
			if err != nil {
				err = fmt.Errorf("failed to decode sae app's env variables, error: %s", err)
				log.Error(err)
				return nil, err
			}

			for _, saeKV := range saeKVMap {
				kv = append(kv, saeKV)
			}

			// if this is the default
			if isSelectedEnv {
				if defaultServiceMap.Has(fmt.Sprintf("%s++%s", serviceName, serviceModule)) {
					selectedServiceList = append(selectedServiceList, &commonmodels.SAEDeployServiceInfo{
						AppID:         tea.StringValue(saeApp.AppId),
						AppName:       tea.StringValue(saeApp.AppName),
						Image:         tea.StringValue(saeApp.ImageUrl),
						ServiceName:   serviceName,
						ServiceModule: serviceModule,
						Envs:          kv,
					})
				}
			}
		}
	}
	return
}
