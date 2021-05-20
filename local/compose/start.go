/*
   Copyright 2020 Docker Compose CLI authors

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

package compose

import (
	"context"

	"github.com/docker/compose-cli/api/compose"
	"github.com/docker/compose-cli/utils"

	"github.com/compose-spec/compose-go/types"
	moby "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/sirupsen/logrus"
)

func (s *composeService) Start(ctx context.Context, project *types.Project, options compose.StartOptions) error {
	if len(options.Services) == 0 {
		options.Services = project.ServiceNames()
	}

	var containers Containers
	if options.Attach != nil {
		c, err := s.attach(ctx, project, options.Attach, options.Services)
		if err != nil {
			return err
		}
		containers = c
	}

	err := InDependencyOrder(ctx, project, func(c context.Context, service types.ServiceConfig) error {
		if utils.StringContains(options.Services, service.Name) {
			return s.startService(ctx, project, service)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if options.Attach == nil {
		return nil
	}

	for _, c := range containers {
		c := c
		go func() {
			s.waitContainer(c, options.Attach)
		}()
	}
	return nil
}

func (s *composeService) waitContainer(c moby.Container, listener compose.ContainerEventListener) {
	statusC, errC := s.apiClient.ContainerWait(context.Background(), c.ID, container.WaitConditionNotRunning)
	name := getContainerNameWithoutProject(c)
	select {
	case status := <-statusC:
		listener(compose.ContainerEvent{
			Type:      compose.ContainerEventExit,
			Container: name,
			Service:   c.Labels[serviceLabel],
			ExitCode:  int(status.StatusCode),
		})
	case err := <-errC:
		logrus.Warnf("Unexpected API error for %s : %s", name, err.Error())
	}
}
