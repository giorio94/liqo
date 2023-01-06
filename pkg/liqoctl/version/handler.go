// Copyright 2019-2023 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package version

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/install"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
)

var liqoctlVersion = "unknown"

// Options encapsulates the arguments of the version command.
type Options struct {
	*factory.Factory

	ClientOnly bool
}

// Run implements the version command.
func (o *Options) Run(ctx context.Context) error {
	fmt.Printf("Client version: %s\n", liqoctlVersion)

	if o.ClientOnly {
		return nil
	}

	release, err := o.HelmClient().GetRelease(install.LiqoReleaseName)
	if err != nil {
		o.Printer.Error.Printfln("Failed to retrieve release information from namespace %q: %v", o.LiqoNamespace, output.PrettyErr(err))
		return err
	}

	if release.Chart == nil || release.Chart.Metadata == nil {
		o.Printer.Error.Println("Invalid release information")
		return err
	}

	version := release.Chart.Metadata.AppVersion
	if version == "" {
		// Development version, fallback to the value specified as tag
		tag, ok := release.Config["tag"]
		if !ok {
			o.Printer.Error.Println("Invalid release information")
			return err
		}
		version = tag.(string)
	}

	fmt.Printf("Server version: %s\n", version)
	return nil
}

func (o *Options) ServerVersionFromHelm(ctx context.Context) (string, error) {
	release, err := o.HelmClient().GetRelease(install.LiqoReleaseName)
	if err != nil {
		return "", err
	}

	if release.Chart == nil || release.Chart.Metadata == nil {
		return "", err
	}

	version := release.Chart.Metadata.AppVersion
	if version == "" {
		// Development version, fallback to the value specified as tag
		tag, ok := release.Config["tag"]
		if !ok {
			o.Printer.Error.Println("Invalid release information")
			return "", err /* TODO: fix */
		}
		version = tag.(string)
	}

	return version, nil
}

func (o *Options) ServerVersionFromDeployment(ctx context.Context) (string, error) {
	// Retrieve the liqo controller manager deployment.
	var deployments appsv1.DeploymentList
	if err := o.CRClient.List(ctx, &deployments, client.InNamespace(o.LiqoNamespace), client.MatchingLabelsSelector{
		Selector: liqolabels.ControllerManagerLabelSelector(),
	}); err != nil || len(deployments.Items) != 1 {
		return "", errors.New("failed to retrieve the liqo controller manager deployment")
	}

	dpl := deployments.Items[0]
	if version := dpl.Labels["app.kubernetes.io/version"]; version != "" {
		return version, nil
	}

	for i := range dpl.Spec.Template.Spec.Containers {
		image := dpl.Spec.Template.Spec.Containers[i].Image
		tokens := strings.Split(image, ":")
		if len(tokens) == 2 {
			return tokens[1], nil
		}
	}

	return "", nil
}
