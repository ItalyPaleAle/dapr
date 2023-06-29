/*
Copyright 2023 The Dapr Authors
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

package internal

import (
	"context"
	"errors"
	"fmt"

	"github.com/spiffe/go-spiffe/v2/spiffeid"

	sentryv1pb "github.com/dapr/dapr/pkg/proto/sentry/v1"
	"github.com/dapr/dapr/pkg/validation"
)

// Validate validates the common rules for all requests.
func Validate(_ context.Context, req *sentryv1pb.SignCertificateRequest) (spiffeid.TrustDomain, error) {
	err := errors.Join(
		validation.ValidateSelfHostedAppID(req.Id),
		appIDLessOrEqualTo64Characters(req.Id),
		csrIsRequired(req.CertificateSigningRequest),
		namespaceIsRequired(req.Namespace),
	)
	if err != nil {
		return spiffeid.TrustDomain{}, fmt.Errorf("invalid request: %w", err)
	}

	if len(req.GetTrustDomain()) == 0 {
		// Default to public trust domain if not specified.
		return spiffeid.RequireTrustDomainFromString("public"), nil
	}

	return spiffeid.TrustDomainFromString(req.GetTrustDomain())
}

func appIDLessOrEqualTo64Characters(appID string) error {
	if len(appID) > 64 {
		return errors.New("app ID must be 64 characters or less")
	}
	return nil
}

func csrIsRequired(csr []byte) error {
	if len(csr) == 0 {
		return errors.New("CSR is required")
	}
	return nil
}

func namespaceIsRequired(namespace string) error {
	if len(namespace) == 0 {
		return errors.New("namespace is required")
	}
	return nil
}

// IsControlPlaneComponent returns true if the app ID corresponds to a Dapr control plane component.
// Note: callers must additionally validate the namespace to ensure it matches the one of the Dapr control plane.
func IsControlPlaneComponent(appID string) bool {
	switch appID {
	case "dapr-operator",
		"dapr-placement",
		"dapr-injector",
		"dapr-sentry":
		return true
	default:
		return false
	}
}