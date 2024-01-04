package actors

import (
	"errors"

	"github.com/dapr/dapr/pkg/actors/emmy"
	"github.com/dapr/dapr/pkg/actors/internal"
)

func init() {
	placementProviders["emmy"] = func(config Config) (placementProviderFactory, error) {
		return func(opts internal.ActorsProviderOptions) internal.PlacementService {
			return emmy.NewActorClient(opts)
		}, nil
	}
	remindersProviders["emmy"] = func(config Config, placement internal.PlacementService) (remindersProviderFactory, error) {
		// Using Emmy as reminders provider requires setting Emmy as placement provider too, with the same address
		client, ok := placement.(*emmy.ActorClient)
		if !ok || config.ActorsService != config.RemindersService {
			return nil, errors.New("when using 'emmy' as reminders provider, actors service provider must be set to the same service 'emmy' as well")
		}

		return func(opts internal.ActorsProviderOptions) internal.RemindersProvider {
			// Return the same client
			return client
		}, nil
	}
}
