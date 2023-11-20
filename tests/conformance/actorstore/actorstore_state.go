//go:build conftests || e2e

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

package actorstore

import (
	"context"
	"fmt"
	mrand "math/rand"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	"github.com/dapr/dapr/pkg/actorssvc/components/actorstore"
)

func actorStateTests(store actorstore.Store) func(t *testing.T) {
	return func(t *testing.T) {
		t.Run("Load test data", loadActorStateTestData(store))
		require.False(t, t.Failed(), "Cannot continue if 'Load test data' test has failed")

		testData := GetTestData()
		var addedHostID string

		t.Run("Add new host", func(t *testing.T) {
			t.Run("Adding new hosts should purge expired ones", func(t *testing.T) {
				before, err := store.GetAllHosts()
				require.NoError(t, err)

				// Add
				addedHostID, err = store.AddActorHost(context.Background(), actorstore.AddActorHostRequest{
					AppID:   "newapp1",
					Address: "10.10.10.10",
					ActorTypes: []actorstore.ActorHostType{
						{ActorType: "newtype1", IdleTimeout: 20},
					},
					APILevel: 10,
				})
				require.NoError(t, err)
				require.NotEmpty(t, addedHostID)

				// Verify
				after, err := store.GetAllHosts()
				require.NoError(t, err)

				require.Len(t, after[addedHostID].ActorTypes, 1)
				require.Equal(t, 20*time.Second, after[addedHostID].ActorTypes["newtype1"].IdleTimeout)

				// 50d7623f-b165-4f9e-9f05-3b7a1280b222 should have been deleted because its last healthcheck was before the interval
				// The newly-added item should be in its place
				// Also note that deleting the actor host should have removed all actors that were hosted by this host
				// If that weren't the case, `GetAllHosts` should have returned an error
				expectHosts := maps.Keys(before)
				for i, v := range expectHosts {
					if v == "50d7623f-b165-4f9e-9f05-3b7a1280b222" {
						expectHosts[i] = addedHostID
					}
				}

				afterHosts := maps.Keys(after)
				slices.Sort(expectHosts)
				slices.Sort(afterHosts)
				require.Equal(t, expectHosts, afterHosts)
			})

			t.Run("Cannot register host with same address", func(t *testing.T) {
				before, err := store.GetAllHosts()
				require.NoError(t, err)

				// Add a host with the same address, just different appID
				_, err = store.AddActorHost(context.Background(), actorstore.AddActorHostRequest{
					AppID:   "newapp2",
					Address: "10.10.10.10",
					ActorTypes: []actorstore.ActorHostType{
						{ActorType: "newtype2", IdleTimeout: 10},
					},
					APILevel: 10,
				})
				require.Error(t, err)
				require.ErrorIs(t, err, actorstore.ErrActorHostConflict)

				// Verify - nothing should have changed
				after, err := store.GetAllHosts()
				require.NoError(t, err)
				require.Equal(t, before, after)
			})

			t.Run("Registering host with API level lower than the cluster's fails", func(t *testing.T) {
				before, err := store.GetAllHosts()
				require.NoError(t, err)

				// Add a host with the same address, just different appID
				_, err = store.AddActorHost(context.Background(), actorstore.AddActorHostRequest{
					AppID:   "newapp-level",
					Address: "11.11.11.11",
					ActorTypes: []actorstore.ActorHostType{
						{ActorType: "newtype", IdleTimeout: 10},
					},
					APILevel: 5,
				})
				require.Error(t, err)
				require.ErrorIs(t, err, actorstore.ErrActorHostAPILevelTooLow)

				// Verify - nothing should have changed
				after, err := store.GetAllHosts()
				require.NoError(t, err)
				require.Equal(t, before, after)
			})
		})

		t.Run("Update existing host", func(t *testing.T) {
			// Ensure clock advances
			require.NoError(t, store.AdvanceTime(500*time.Millisecond))

			t.Run("Update actor types", func(t *testing.T) {
				before, err := store.GetAllHosts()
				require.NoError(t, err)

				err = store.UpdateActorHost(context.Background(), addedHostID, actorstore.UpdateActorHostRequest{
					// Do not update last health check
					UpdateLastHealthCheck: false,
					ActorTypes: []actorstore.ActorHostType{
						{ActorType: "newtype", IdleTimeout: 10},
					},
				})
				require.NoError(t, err)

				// Verify
				after, err := store.GetAllHosts()
				require.NoError(t, err)

				expect := before[addedHostID]
				expect.ActorTypes = map[string]actorstore.TestDataActorType{
					"newtype": {
						IdleTimeout: 10 * time.Second,
						ActorIDs:    []string{},
					},
				}

				require.Equal(t, expect, after[addedHostID])
			})

			t.Run("Update host last healthcheck", func(t *testing.T) {
				before, err := store.GetAllHosts()
				require.NoError(t, err)

				err = store.UpdateActorHost(context.Background(), addedHostID, actorstore.UpdateActorHostRequest{
					UpdateLastHealthCheck: true,
				})
				require.NoError(t, err)

				// Verify
				after, err := store.GetAllHosts()
				require.NoError(t, err)

				require.Equal(t, before[addedHostID].AppID, after[addedHostID].AppID)
				require.Equal(t, before[addedHostID].Address, after[addedHostID].Address)
				require.Equal(t, before[addedHostID].ActorTypes, after[addedHostID].ActorTypes)
				require.True(t, before[addedHostID].LastHealthCheck.Before(after[addedHostID].LastHealthCheck))
			})

			t.Run("Error when host ID is empty", func(t *testing.T) {
				err := store.UpdateActorHost(context.Background(), "", actorstore.UpdateActorHostRequest{
					UpdateLastHealthCheck: true,
				})
				require.Error(t, err)
				require.ErrorIs(t, err, actorstore.ErrInvalidRequestMissingParameters)
			})

			t.Run("Error when nothing to update", func(t *testing.T) {
				err := store.UpdateActorHost(context.Background(), "d0d8b4c1-0b34-4e8e-9163-a8ac72c4a0d6", actorstore.UpdateActorHostRequest{})
				require.Error(t, err)
				require.ErrorIs(t, err, actorstore.ErrInvalidRequestMissingParameters)
			})

			t.Run("Error when host doesn't exist", func(t *testing.T) {
				err := store.UpdateActorHost(context.Background(), "d0d8b4c1-0b34-4e8e-9163-a8ac72c4a0d6", actorstore.UpdateActorHostRequest{
					UpdateLastHealthCheck: true,
				})
				require.Error(t, err)
				require.ErrorIs(t, err, actorstore.ErrActorHostNotFound)
			})
		})

		t.Run("Remove actor host", func(t *testing.T) {
			t.Run("Remove existing host", func(t *testing.T) {
				before, err := store.GetAllHosts()
				require.NoError(t, err)

				err = store.RemoveActorHost(context.Background(), addedHostID)
				require.NoError(t, err)

				// Verify
				after, err := store.GetAllHosts()
				require.NoError(t, err)

				require.Len(t, after, len(before)-1)
				require.Empty(t, after[addedHostID])
			})

			t.Run("Error when host ID is empty", func(t *testing.T) {
				err := store.RemoveActorHost(context.Background(), "")
				require.Error(t, err)
				require.ErrorIs(t, err, actorstore.ErrInvalidRequestMissingParameters)
			})

			t.Run("Error when host doesn't exist", func(t *testing.T) {
				err := store.RemoveActorHost(context.Background(), "d0d8b4c1-0b34-4e8e-9163-a8ac72c4a0d6")
				require.Error(t, err)
				require.ErrorIs(t, err, actorstore.ErrActorHostNotFound)
			})
		})

		t.Run("Lookup actor", func(t *testing.T) {
			hosts, err := store.GetAllHosts()
			require.NoError(t, err)

			t.Run("No host limit", func(t *testing.T) {
				t.Run("Reload test data", loadActorStateTestData(store))

				t.Run("Active actor", func(t *testing.T) {
					// Test vectors: key is "actor-type/actor-id" and value is expected host ID
					tt := map[string]string{
						"type-B/type-B.211": "ded1e507-ed4a-4322-a3a4-b5e8719a9333",
						"type-C/type-C.12":  "ded1e507-ed4a-4322-a3a4-b5e8719a9333",
						"type-B/type-B.222": "f4c7d514-3468-48dd-9103-297bf7fe91fd",
						"type-A/type-A.11":  "7de434ce-e285-444f-9857-4d30cade3111",
						"type-A/type-A.12":  "7de434ce-e285-444f-9857-4d30cade3111",
						"type-B/type-B.112": "7de434ce-e285-444f-9857-4d30cade3111",
					}

					for k, v := range tt {
						ref := actorstore.ActorRef{}
						ref.ActorType, ref.ActorID, _ = strings.Cut(k, "/")
						res, err := store.LookupActor(context.Background(), ref, actorstore.LookupActorOpts{})
						require.NoErrorf(t, err, "Error on key %s", k)

						require.Equalf(t, v, res.HostID, "Error on key %s", k)
						require.Equalf(t, hosts[v].AppID, res.AppID, "Error on key %s", k)
						require.Equalf(t, hosts[v].Address, res.Address, "Error on key %s", k)
						require.EqualValuesf(t, hosts[v].ActorTypes[ref.ActorType].IdleTimeout.Seconds(), res.IdleTimeout, "Error on key %s", k)
					}
				})

				t.Run("Inactive actor", func(t *testing.T) {
					const iterationsPerActorType = 50
					for at, atHosts := range testData.HostsByActorType(store.GetTime(), configHostHealthCheckInterval) {
						t.Run(at, func(t *testing.T) {
							counts := map[string]int{}
							for _, host := range atHosts {
								counts[host] = 0
							}

							for i := 0; i < iterationsPerActorType; i++ {
								res, err := store.LookupActor(context.Background(), actorstore.ActorRef{
									ActorType: at,
									ActorID:   fmt.Sprintf("inactive-%d", i),
								}, actorstore.LookupActorOpts{})
								require.NoErrorf(t, err, "Failed on iteration %s/%d", at, i)

								require.Containsf(t, atHosts, res.HostID, "Failed on iteration %s/%d", at, i)
								require.Equalf(t, hosts[res.HostID].AppID, res.AppID, "Failed on iteration %s/%d", at, i)
								require.Equalf(t, hosts[res.HostID].Address, res.Address, "Failed on iteration %s/%d", at, i)
								require.EqualValuesf(t, hosts[res.HostID].ActorTypes[at].IdleTimeout.Seconds(), res.IdleTimeout, "Failed on iteration %s/%d", at, i)

								counts[res.HostID]++
							}

							// Ideally we'd have a perfectly uniform distribution of actors across all hosts
							// But this isn't always the case, so we will only assert that at least 1/3rd of what would be uniform is assigned to each host
							for host, count := range counts {
								min := (iterationsPerActorType / len(counts)) / 3
								if min < 1 {
									min = 1
								}
								assert.GreaterOrEqualf(t, count, min, "Failed on host %s", host)
							}
						})
					}
				})

				t.Run("Actor is active on unhealthy host", func(t *testing.T) {
					// Host 50d7623f-b165-4f9e-9f05-3b7a1280b222 is inactive
					actorTypes := testData.Hosts["50d7623f-b165-4f9e-9f05-3b7a1280b222"].ActorTypes
					for actorType, v := range actorTypes {
						for _, actorID := range v.ActorIDs {
							res, err := store.LookupActor(context.Background(), actorstore.ActorRef{
								ActorType: actorType,
								ActorID:   actorID,
							}, actorstore.LookupActorOpts{})
							require.NoErrorf(t, err, "Failed on iteration %s/%s", actorType, actorID)
							require.NotEqual(t, "50d7623f-b165-4f9e-9f05-3b7a1280b222", res.HostID)
						}
					}
				})

				t.Run("No host for actor type", func(t *testing.T) {
					_, err := store.LookupActor(context.Background(), actorstore.ActorRef{
						ActorType: "not-supported",
						ActorID:   "1",
					}, actorstore.LookupActorOpts{})
					require.Error(t, err)
					assert.ErrorIs(t, err, actorstore.ErrNoActorHost)
				})
			})

			t.Run("With host limit", func(t *testing.T) {
				lookupOpts := actorstore.LookupActorOpts{
					Hosts: []string{
						// Limit lookups to these two hosts
						// These can hosts actors of type type-B and type-C only
						"ded1e507-ed4a-4322-a3a4-b5e8719a9333",
						"f4c7d514-3468-48dd-9103-297bf7fe91fd",
					},
				}

				t.Run("Active actor", func(t *testing.T) {
					// Test vectors: key is "actor-type/actor-id" and value is expected host ID
					tt := map[string]string{
						"type-B/type-B.211": "ded1e507-ed4a-4322-a3a4-b5e8719a9333",
						"type-C/type-C.12":  "ded1e507-ed4a-4322-a3a4-b5e8719a9333",
						"type-B/type-B.222": "f4c7d514-3468-48dd-9103-297bf7fe91fd",
						"type-A/type-A.11":  "7de434ce-e285-444f-9857-4d30cade3111",
						"type-A/type-A.12":  "7de434ce-e285-444f-9857-4d30cade3111",
						"type-B/type-B.112": "7de434ce-e285-444f-9857-4d30cade3111",
					}

					for k, v := range tt {
						ref := actorstore.ActorRef{}
						ref.ActorType, ref.ActorID, _ = strings.Cut(k, "/")
						res, err := store.LookupActor(context.Background(), ref, lookupOpts)

						if slices.Contains(lookupOpts.Hosts, v) {
							require.NoErrorf(t, err, "Error on key %s", k)

							require.Equalf(t, v, res.HostID, "Error on key %s", k)
							require.Equalf(t, hosts[v].AppID, res.AppID, "Error on key %s", k)
							require.Equalf(t, hosts[v].Address, res.Address, "Error on key %s", k)
							require.EqualValuesf(t, hosts[v].ActorTypes[ref.ActorType].IdleTimeout.Seconds(), res.IdleTimeout, "Error on key %s", k)
						} else {
							require.Errorf(t, err, "Error on key %s", k)
							assert.ErrorIsf(t, err, actorstore.ErrNoActorHost, "Error on key %s", k)
						}
					}
				})

				t.Run("Inactive actor", func(t *testing.T) {
					const iterationsPerActorType = 50
					for at, atHosts := range testData.HostsByActorType(store.GetTime(), configHostHealthCheckInterval) {
						t.Run(at, func(t *testing.T) {
							counts := map[string]int{}
							for _, host := range atHosts {
								if slices.Contains(lookupOpts.Hosts, host) {
									counts[host] = 0
								}
							}

							expectErr := len(counts) == 0

							for i := 0; i < iterationsPerActorType; i++ {
								res, err := store.LookupActor(context.Background(), actorstore.ActorRef{
									ActorType: at,
									ActorID:   fmt.Sprintf("inactive-opts-%d", i),
								}, lookupOpts)

								if expectErr {
									require.Errorf(t, err, "Failed on iteration %s/%d", at, i)
									assert.ErrorIsf(t, err, actorstore.ErrNoActorHost, "Failed on iteration %s/%d", at, i)
								} else {
									require.NoErrorf(t, err, "Failed on iteration %s/%d", at, i)

									require.Containsf(t, atHosts, res.HostID, "Failed on iteration %s/%d", at, i)
									require.Containsf(t, lookupOpts.Hosts, res.HostID, "Failed on iteration %s/%d", at, i)
									require.Equalf(t, hosts[res.HostID].AppID, res.AppID, "Failed on iteration %s/%d", at, i)
									require.Equalf(t, hosts[res.HostID].Address, res.Address, "Failed on iteration %s/%d", at, i)
									require.EqualValuesf(t, hosts[res.HostID].ActorTypes[at].IdleTimeout.Seconds(), res.IdleTimeout, "Failed on iteration %s/%d", at, i)

									counts[res.HostID]++
								}
							}

							if !expectErr {
								// Ideally we'd have a perfectly uniform distribution of actors across all hosts
								// But this isn't always the case, so we will only assert that at least 1/3rd of what would be uniform is assigned to each host
								for host, count := range counts {
									min := (iterationsPerActorType / len(counts)) / 3
									if min < 1 {
										min = 1
									}
									assert.GreaterOrEqualf(t, count, min, "Failed on host %s", host)
								}
							}
						})
					}
				})

				t.Run("No host for actor type", func(t *testing.T) {
					_, err := store.LookupActor(context.Background(), actorstore.ActorRef{
						ActorType: "not-supported",
						ActorID:   "1",
					}, actorstore.LookupActorOpts{})
					require.Error(t, err)
					assert.ErrorIs(t, err, actorstore.ErrNoActorHost)
				})
			})

			t.Run("Parallel lookups", func(t *testing.T) {
				testParallelLookups := func(restrictToHosts []string) func(t *testing.T) {
					return func(t *testing.T) {
						// This is a stress test that invokes LookupActor multiple times, in parallel, also repeating some actor IDs
						const iterationsPerActorType = 150
						hostsByActorTypes := testData.HostsByActorType(store.GetTime(), configHostHealthCheckInterval)
						tt := make([]string, 0, (len(hostsByActorTypes)*iterationsPerActorType)+26)
						for at := range hostsByActorTypes {
							for j := 0; j < iterationsPerActorType; j++ {
								// Some actor IDs will be repeated, and that's by design
								num := mrand.Intn(iterationsPerActorType * 0.7) //nolint:gosec
								key := fmt.Sprintf("%s/parallel-%d", at, num)
								tt = append(tt, key)
							}
						}

						// Add some additional actors that are already active, and some that are active on an unhealthy host
						tt = append(tt,
							// Active
							"type-B/type-B.223",
							"type-B/type-B.223",
							"type-B/type-B.224",
							"type-B/type-B.224",
							"type-B/type-B.224",
							"type-A/type-A.11",
							"type-A/type-A.11",
							"type-A/type-A.11",
							"type-A/type-A.13",
							"type-A/type-A.13",
							"type-A/type-A.13",
							"type-B/type-B.111",
							"type-B/type-B.111",
							"type-B/type-B.111",
							// Active but on unhealthy host
							"type-A/type-A.21",
							"type-A/type-A.21",
							"type-A/type-A.21",
							"type-A/type-A.21",
							"type-A/type-A.22",
							"type-A/type-A.22",
							"type-A/type-A.22",
							"type-A/type-A.22",
							"type-B/type-B.121",
							"type-B/type-B.121",
							"type-B/type-B.121",
							"type-B/type-B.121",
						)

						// Shuffle
						mrand.Shuffle(len(tt), func(i, j int) {
							tt[i], tt[j] = tt[j], tt[i]
						})

						// Set lookup options
						lookupOpts := actorstore.LookupActorOpts{
							Hosts: restrictToHosts,
						}

						// Start the requests in parallel
						type result struct {
							key    string
							err    error
							hostID string
						}
						results := make(chan result)
						for i := 0; i < len(tt); i++ {
							go func(key string) {
								var ref actorstore.ActorRef
								ref.ActorType, ref.ActorID, _ = strings.Cut(key, "/")
								lar, err := store.LookupActor(context.Background(), ref, lookupOpts)
								res := result{
									key: key,
								}
								if err != nil {
									res.err = err
									results <- res
									return
								}
								res.hostID = lar.HostID
								results <- res
							}(tt[i])
						}

						// Read the results
						collectedHostIDs := make(map[string]string, int(float64(len(tt))*0.9))
						collected := make([]string, len(tt))
						for i := 0; i < len(tt); i++ {
							res := <-results
							if !assert.NoErrorf(t, res.err, "Error returned for key %s", res.key) {
								continue
							}

							collected[i] = res.key
							if collectedHostIDs[res.key] != "" {
								assert.Equalf(t, res.hostID, collectedHostIDs[res.key], "Unexpected response for key %s", res.key)
							} else {
								collectedHostIDs[res.key] = res.hostID
							}

							if len(restrictToHosts) > 0 {
								assert.Containsf(t, restrictToHosts, res.hostID, "Response for key %s was a restricted host", res.key)
							}
						}

						// Ensure we have a response for all requests
						slices.Sort(collected)
						slices.Sort(tt)
						assert.Equal(t, tt, collected)

						// Check that certain known actors have expected values
						// These actors were already active on a healthy host
						assert.Equal(t, "f4c7d514-3468-48dd-9103-297bf7fe91fd", collectedHostIDs["type-B/type-B.223"])
						assert.Equal(t, "f4c7d514-3468-48dd-9103-297bf7fe91fd", collectedHostIDs["type-B/type-B.224"])
						assert.Equal(t, "7de434ce-e285-444f-9857-4d30cade3111", collectedHostIDs["type-A/type-A.11"])
						assert.Equal(t, "7de434ce-e285-444f-9857-4d30cade3111", collectedHostIDs["type-A/type-A.13"])
						assert.Equal(t, "7de434ce-e285-444f-9857-4d30cade3111", collectedHostIDs["type-B/type-B.111"])
						// These actors were already active, but on an unhealthy host
						assert.NotEqual(t, "50d7623f-b165-4f9e-9f05-3b7a1280b222", collectedHostIDs["type-A/type-A.21"])
						assert.NotEqual(t, "50d7623f-b165-4f9e-9f05-3b7a1280b222", collectedHostIDs["type-A/type-A.22"])
						assert.NotEqual(t, "50d7623f-b165-4f9e-9f05-3b7a1280b222", collectedHostIDs["type-B/type-B.121"])
					}
				}

				// Reload test data before any run
				loadActorStateTestData(store)(t)
				t.Run("Test without host restrictions", testParallelLookups(nil))

				loadActorStateTestData(store)(t)
				t.Run("Test with host restrictions", testParallelLookups([]string{"f4c7d514-3468-48dd-9103-297bf7fe91fd", "7de434ce-e285-444f-9857-4d30cade3111"}))
			})

			t.Run("Error when actor type is empty", func(t *testing.T) {
				_, err := store.LookupActor(context.Background(), actorstore.ActorRef{
					ActorType: "",
					ActorID:   "id",
				}, actorstore.LookupActorOpts{})
				require.Error(t, err)
				assert.ErrorIs(t, err, actorstore.ErrInvalidRequestMissingParameters)
			})

			t.Run("Error when actor ID is empty", func(t *testing.T) {
				_, err := store.LookupActor(context.Background(), actorstore.ActorRef{
					ActorType: "type",
					ActorID:   "",
				}, actorstore.LookupActorOpts{})
				require.Error(t, err)
				assert.ErrorIs(t, err, actorstore.ErrInvalidRequestMissingParameters)
			})
		})

		t.Run("Remove actor", func(t *testing.T) {
			t.Run("Remove existing actor", func(t *testing.T) {
				before, err := store.GetAllHosts()
				require.NoError(t, err)

				// This actor is hosted on ded1e507-ed4a-4322-a3a4-b5e8719a9333
				const (
					hostID    = "ded1e507-ed4a-4322-a3a4-b5e8719a9333"
					actorType = "type-B"
					actorID   = "type-B.211"
				)
				err = store.RemoveActor(context.Background(), actorstore.ActorRef{
					ActorType: actorType,
					ActorID:   actorID,
				})
				require.NoError(t, err)

				// Verify
				after, err := store.GetAllHosts()
				require.NoError(t, err)

				require.Len(t, after[hostID].ActorTypes[actorType].ActorIDs, len(before[hostID].ActorTypes[actorType].ActorIDs)-1)
				require.NotContains(t, after[hostID].ActorTypes[actorType].ActorIDs, actorID)
			})
		})
	}
}
