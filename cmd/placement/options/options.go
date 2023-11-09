/*
Copyright 2021 The Dapr Authors
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

package options

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"

	"github.com/dapr/dapr/pkg/metrics"
	"github.com/dapr/dapr/pkg/modes"
	"github.com/dapr/dapr/pkg/placement/raft"
	"github.com/dapr/dapr/pkg/security"
	securityConsts "github.com/dapr/dapr/pkg/security/consts"
	"github.com/dapr/kit/logger"
	"github.com/dapr/kit/utils"
)

const (
	//nolint:gosec
	defaultCredentialsPath   = "/var/run/dapr/credentials"
	defaultHealthzPort       = 8080
	defaultPlacementPort     = 50005
	defaultReplicationFactor = 100
	envMetadataEnabled       = "DAPR_PLACEMENT_METADATA_ENABLED"
)

type Options struct {
	// Raft protocol configurations
	RaftID           string
	raftPeerFlag     []string
	RaftPeers        []raft.PeerInfo
	RaftInMemEnabled bool
	RaftLogStorePath string

	// Placement server configurations
	PlacementPort   int
	HealthzPort     int
	MetadataEnabled bool

	TLSEnabled       bool
	TrustDomain      string
	TrustAnchorsFile string
	SentryAddress    string
	Mode             string

	ReplicationFactor int

	// Log and metrics configurations
	Logger  logger.Options
	Metrics *metrics.Options
}

func New(args []string) *Options {
	// Default options
	opts := Options{
		MetadataEnabled: utils.IsTruthy(os.Getenv(envMetadataEnabled)),
	}

	// Create a flag set
	fs := pflag.NewFlagSet("sentry", pflag.ExitOnError)
	fs.SortFlags = true

	fs.StringVar(&opts.RaftID, "id", "dapr-placement-0", "Placement server ID")
	fs.StringSliceVar(&opts.raftPeerFlag, "initial-cluster", []string{"dapr-placement-0=127.0.0.1:8201"}, "raft cluster peers")
	fs.BoolVar(&opts.RaftInMemEnabled, "inmem-store-enabled", true, "Enable in-memory log and snapshot store unless --raft-logstore-path is set")
	fs.StringVar(&opts.RaftLogStorePath, "raft-logstore-path", "", "raft log store path.")
	fs.IntVar(&opts.PlacementPort, "port", defaultPlacementPort, "sets the gRPC port for the placement service")
	fs.IntVar(&opts.HealthzPort, "healthz-port", defaultHealthzPort, "sets the HTTP port for the healthz server")
	fs.BoolVar(&opts.TLSEnabled, "tls-enabled", false, "Should TLS be enabled for the placement gRPC server")
	fs.BoolVar(&opts.MetadataEnabled, "metadata-enabled", opts.MetadataEnabled, "Expose the placement tables on the healthz server")
	fs.IntVar(&opts.ReplicationFactor, "replicationFactor", defaultReplicationFactor, "sets the replication factor for actor distribution on vnodes")

	fs.StringVar(&opts.TrustDomain, "trust-domain", "localhost", "Trust domain for the Dapr control plane")
	fs.StringVar(&opts.TrustAnchorsFile, "trust-anchors-file", securityConsts.ControlPlaneDefaultTrustAnchorsPath, "Filepath to the trust anchors for the Dapr control plane")
	fs.StringVar(&opts.SentryAddress, "sentry-address", fmt.Sprintf("dapr-sentry.%s.svc:443", security.CurrentNamespace()), "Address of the Sentry service")
	fs.StringVar(&opts.Mode, "mode", string(modes.StandaloneMode), "Runtime mode for Placement")

	opts.Logger = logger.DefaultOptions()
	opts.Logger.AttachCmdFlags(fs.StringVar, fs.BoolVar)

	opts.Metrics = metrics.DefaultMetricOptions()
	opts.Metrics.AttachCmdFlags(fs.StringVar, fs.BoolVar)

	// Ignore errors; flagset is set for ExitOnError
	_ = fs.Parse(args)

	opts.RaftPeers = parsePeersFromFlag(opts.raftPeerFlag)
	if opts.RaftLogStorePath != "" {
		opts.RaftInMemEnabled = false
	}

	return &opts
}

func parsePeersFromFlag(val []string) []raft.PeerInfo {
	peers := make([]raft.PeerInfo, len(val))

	i := 0
	for _, addr := range val {
		peer := strings.SplitN(addr, "=", 3)
		if len(peer) != 2 {
			continue
		}

		peers[i] = raft.PeerInfo{
			ID:      strings.TrimSpace(peer[0]),
			Address: strings.TrimSpace(peer[1]),
		}
		i++
	}

	return peers[:i]
}
