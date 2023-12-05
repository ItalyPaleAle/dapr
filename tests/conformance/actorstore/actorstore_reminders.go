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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapr/dapr/pkg/actorssvc/components/actorstore"
	"github.com/dapr/kit/ptr"
)

func remindersTest(store actorstore.Store) func(t *testing.T) {
	return func(t *testing.T) {
		t.Run("CRUD operations", func(t *testing.T) {
			// Load reminder test data
			t.Run("Load test data", func(t *testing.T) {
				require.NoError(t, store.LoadReminderTestData(GetTestData()), "Failed to load reminder test data")
			})
			require.False(t, t.Failed(), "Cannot continue if 'Load test data' test has failed")

			testData := GetTestData()

			t.Run("Get a reminder", func(t *testing.T) {
				t.Run("Retrieve an existing reminder", func(t *testing.T) {
					r := testData.Reminders["f647315e-ffeb-4727-8a7a-539bb0d3e3cc"]
					res, err := store.GetReminder(context.Background(), actorstore.ReminderRef{
						ActorType: r.ActorType,
						ActorID:   r.ActorID,
						Name:      r.Name,
					})

					require.NoError(t, err)
					assert.InDelta(t, store.GetTime().Add(r.ExecutionTime).UnixNano(), res.ExecutionTime.UnixNano(), float64(time.Second/2))
				})

				t.Run("Error when reminder doesn't exist", func(t *testing.T) {
					_, err := store.GetReminder(context.Background(), actorstore.ReminderRef{
						ActorType: "notfound",
						ActorID:   "notfound",
						Name:      "notfound",
					})
					require.Error(t, err)
					require.ErrorIs(t, err, actorstore.ErrReminderNotFound)
				})

				t.Run("Error when missing ActorType", func(t *testing.T) {
					_, err := store.GetReminder(context.Background(), actorstore.ReminderRef{
						ActorType: "",
						ActorID:   "myactorid",
						Name:      "myreminder",
					})
					require.Error(t, err)
					require.ErrorIs(t, err, actorstore.ErrInvalidRequestMissingParameters)
				})

				t.Run("Error when missing ActorID", func(t *testing.T) {
					_, err := store.GetReminder(context.Background(), actorstore.ReminderRef{
						ActorType: "myactortype",
						ActorID:   "",
						Name:      "myreminder",
					})
					require.Error(t, err)
					require.ErrorIs(t, err, actorstore.ErrInvalidRequestMissingParameters)
				})

				t.Run("Error when missing Name", func(t *testing.T) {
					_, err := store.GetReminder(context.Background(), actorstore.ReminderRef{
						ActorType: "myactortype",
						ActorID:   "myactorid",
						Name:      "",
					})
					require.Error(t, err)
					require.ErrorIs(t, err, actorstore.ErrInvalidRequestMissingParameters)
				})
			})

			t.Run("Create a reminder", func(t *testing.T) {
				t.Run("Create a new reminder with fixed execution time, period, and data", func(t *testing.T) {
					ref := actorstore.ReminderRef{
						ActorType: "type-D",
						ActorID:   "type-D.1",
						Name:      "type-D.1.1",
					}
					opts := actorstore.ReminderOptions{
						ExecutionTime: store.GetTime().Add(time.Minute),
						Period:        ptr.Of("1m"),
						Data:          []byte("almeno tu nell'universo"),
					}

					err := store.CreateReminder(context.Background(), actorstore.Reminder{
						ReminderRef:     ref,
						ReminderOptions: opts,
					})
					require.NoError(t, err)

					res, err := store.GetReminder(context.Background(), ref)
					require.NoError(t, err)
					if assert.NotNil(t, res.Period) {
						assert.Equal(t, *opts.Period, *res.Period)
					}
					assert.Nil(t, res.TTL)
					assert.Equal(t, opts.Data, res.Data)
					assert.InDelta(t, opts.ExecutionTime.UnixNano(), res.ExecutionTime.UnixNano(), float64(time.Second/2))
				})

				t.Run("Create a new reminder with a delay and TTL", func(t *testing.T) {
					now := store.GetTime()
					ttl := now.Add(time.Hour)
					ref := actorstore.ReminderRef{
						ActorType: "type-D",
						ActorID:   "type-D.1",
						Name:      "type-D.1.2",
					}
					opts := actorstore.ReminderOptions{
						Delay: time.Minute,
						TTL:   &ttl,
					}

					err := store.CreateReminder(context.Background(), actorstore.Reminder{
						ReminderRef:     ref,
						ReminderOptions: opts,
					})
					require.NoError(t, err)

					res, err := store.GetReminder(context.Background(), ref)
					require.NoError(t, err)
					if assert.NotNil(t, res.TTL) {
						assert.InDelta(t, ttl.UnixNano(), res.TTL.UnixNano(), float64(time.Second/2))
					}
					assert.Nil(t, res.Period)
					assert.Empty(t, res.Data)
					assert.InDelta(t, now.Add(opts.Delay).UnixNano(), res.ExecutionTime.UnixNano(), float64(time.Second/2))
				})

				t.Run("Create a reminder with 0s delay", func(t *testing.T) {
					now := store.GetTime()
					ref := actorstore.ReminderRef{
						ActorType: "type-D",
						ActorID:   "type-D.1",
						Name:      "type-D.1.3",
					}

					err := store.CreateReminder(context.Background(), actorstore.Reminder{
						ReminderRef:     ref,
						ReminderOptions: actorstore.ReminderOptions{
							// Empty: no ExecutionTime and no Delay
							// Equals to 0 delay
						},
					})
					require.NoError(t, err)

					res, err := store.GetReminder(context.Background(), ref)
					require.NoError(t, err)
					assert.Nil(t, res.TTL)
					assert.Nil(t, res.Period)
					assert.Empty(t, res.Data)
					assert.InDelta(t, now.UnixNano(), res.ExecutionTime.UnixNano(), float64(time.Second/2))
				})

				t.Run("Replace an existing reminder", func(t *testing.T) {
					now := store.GetTime()
					ref := actorstore.ReminderRef{
						ActorType: "type-D",
						ActorID:   "type-D.1",
						Name:      "type-D.1.1",
					}
					opts := actorstore.ReminderOptions{
						// Should delete period and data
						Delay: 2 * time.Minute,
					}

					err := store.CreateReminder(context.Background(), actorstore.Reminder{
						ReminderRef:     ref,
						ReminderOptions: opts,
					})
					require.NoError(t, err)

					res, err := store.GetReminder(context.Background(), ref)
					require.NoError(t, err)
					assert.Nil(t, res.TTL)
					assert.Nil(t, res.Period)
					assert.Empty(t, res.Data)
					assert.InDelta(t, now.Add(opts.Delay).UnixNano(), res.ExecutionTime.UnixNano(), float64(time.Second/2))
				})

				t.Run("Error with invalid ReminderRef", func(t *testing.T) {
					err := store.CreateReminder(context.Background(), actorstore.CreateReminderRequest{
						ReminderRef: actorstore.ReminderRef{
							ActorType: "",
							ActorID:   "myactorid",
							Name:      "myreminder",
						},
						ReminderOptions: actorstore.ReminderOptions{
							Delay: 2 * time.Minute,
						},
					})
					require.Error(t, err)
					require.ErrorIs(t, err, actorstore.ErrInvalidRequestMissingParameters)
				})
			})

			t.Run("Delete a reminder", func(t *testing.T) {
				t.Run("Delete an existing reminder", func(t *testing.T) {
					ref := actorstore.ReminderRef{
						ActorType: "type-D",
						ActorID:   "type-D.1",
						Name:      "type-D.1.1",
					}
					err := store.DeleteReminder(context.Background(), ref)
					require.NoError(t, err)

					_, err = store.GetReminder(context.Background(), ref)
					require.Error(t, err)
					require.ErrorIs(t, err, actorstore.ErrReminderNotFound)
				})

				t.Run("Error when reminder doesn't exist", func(t *testing.T) {
					err := store.DeleteReminder(context.Background(), actorstore.ReminderRef{
						ActorType: "notfound",
						ActorID:   "notfound",
						Name:      "notfound",
					})
					require.Error(t, err)
					require.ErrorIs(t, err, actorstore.ErrReminderNotFound)
				})

				t.Run("Error with invalid ReminderRef", func(t *testing.T) {
					_, err := store.GetReminder(context.Background(), actorstore.ReminderRef{
						ActorType: "",
						ActorID:   "myactorid",
						Name:      "myreminder",
					})
					require.Error(t, err)
					require.ErrorIs(t, err, actorstore.ErrInvalidRequestMissingParameters)
				})
			})
		})

		assertRemindersInResponse := func(t *testing.T, res []*actorstore.FetchedReminder, expect []string, start time.Time) {
			expectLen := len(expect)
			require.Len(t, res, expectLen)
			foundKeys := make([]string, expectLen)
			for i := 0; i < expectLen; i++ {
				foundKeys[i] = res[i].Key()
				assert.NotEmptyf(t, res[i].Lease(), "reminder=%s", res[i].Key())
				if !start.IsZero() {
					assert.GreaterOrEqualf(t, res[i].ScheduledTime().Unix(), start.Unix(), "reminder=%s scheduledTime=%v currentTime=%v", res[i].Key(), res[i].ScheduledTime(), start)
				}
			}

			assert.ElementsMatch(t, expect, foundKeys)
		}

		t.Run("Fetch next reminders", func(t *testing.T) {
			// Reload reminder test data
			t.Run("Load test data", func(t *testing.T) {
				td := GetTestData()
				require.NoError(t, store.LoadActorStateTestData(td), "Failed to load actor store test data")
				require.NoError(t, store.LoadReminderTestData(td), "Failed to load reminder test data")
			})
			require.False(t, t.Failed(), "Cannot continue if 'Load test data' test has failed")

			// Subtract 1s as buffer
			start := store.GetTime().Add(-1 * time.Second)

			// Advance the time by 100ms
			require.NoError(t, store.AdvanceTime(100*time.Millisecond))

			t.Run("Fetching reminders", func(t *testing.T) {
				req := actorstore.FetchNextRemindersRequest{
					// Note that "50d7623f-b165-4f9e-9f05-3b7a1280b222" is unhealthy
					Hosts:      []string{"7de434ce-e285-444f-9857-4d30cade3111", "50d7623f-b165-4f9e-9f05-3b7a1280b222"},
					ActorTypes: []string{"type-A", "type-B"},
				}

				t.Run("Fetch next 2 reminders", func(t *testing.T) {
					res, err := store.FetchNextReminders(context.Background(), req)
					require.NoError(t, err)
					require.NotNil(t, res)

					assertRemindersInResponse(t, res, []string{"type-A||type-A.11||type-A.11.2", "type-A||type-A.11||type-A.11.1"}, start)

					host := getHostForActor(t, store, "type-A", "type-A.11")
					assert.Equal(t, "7de434ce-e285-444f-9857-4d30cade3111", host)
				})

				// No point in continuing if the tests failed
				require.False(t, t.Failed(), "Cannot continue if previous test failed")

				t.Run("One more reminder being retrieved for same hosts", func(t *testing.T) {
					res, err := store.FetchNextReminders(context.Background(), req)
					require.NoError(t, err)
					require.NotNil(t, res)

					assertRemindersInResponse(t, res, []string{"type-A||type-A.inactivereminder||type-A.inactivereminder.1"}, start)

					// Can only be on "7de434ce-e285-444f-9857-4d30cade3111" because "50d7623f-b165-4f9e-9f05-3b7a1280b222" is unhealthy
					host := getHostForActor(t, store, "type-A", "type-A.inactivereminder")
					assert.Equal(t, "7de434ce-e285-444f-9857-4d30cade3111", host)
				})

				// No point in continuing if the tests failed
				require.False(t, t.Failed(), "Cannot continue if previous test failed")

				t.Run("No more reminders", func(t *testing.T) {
					res, err := store.FetchNextReminders(context.Background(), req)
					require.NoError(t, err)
					require.Empty(t, res)
				})

				// No point in continuing if the tests failed
				require.False(t, t.Failed(), "Cannot continue if previous test failed")

				// Advance clock by 1.5s to make more reminders appear
				require.NoError(t, store.AdvanceTime(1500*time.Millisecond))

				t.Run("Fetch next reminders", func(t *testing.T) {
					res, err := store.FetchNextReminders(context.Background(), req)
					require.NoError(t, err)
					require.NotNil(t, res)

					assertRemindersInResponse(t, res, []string{"type-A||type-A.inactivereminder||type-A.inactivereminder.2", "type-A||type-A.inactivereminder||type-A.inactivereminder.3"}, start)

					// Can only be on "7de434ce-e285-444f-9857-4d30cade3111" because "50d7623f-b165-4f9e-9f05-3b7a1280b222" is unhealthy
					host := getHostForActor(t, store, "type-A", "type-A.inactivereminder")
					assert.Equal(t, "7de434ce-e285-444f-9857-4d30cade3111", host)
				})

				// No point in continuing if the tests failed
				require.False(t, t.Failed(), "Cannot continue if previous test failed")

				t.Run("Fetch reminders for another host", func(t *testing.T) {
					// There are 3 reminders that match this request, all overdue
					// Because reminders are fetched in order, we expect the same results
					// (The order of items in the returned slice is not guarnateed, but it's guaranteed that they are the earliest reminders to be executed)
					req = actorstore.FetchNextRemindersRequest{
						Hosts:      []string{"f4c7d514-3468-48dd-9103-297bf7fe91fd"},
						ActorTypes: []string{"type-B", "type-C"},
					}

					res, err := store.FetchNextReminders(context.Background(), req)
					require.NoError(t, err)
					require.NotNil(t, res)
					assertRemindersInResponse(t, res, []string{"type-C||type-C.inactivereminder||type-C.inactivereminder.1", "type-C||type-C.inactivereminder||type-C.inactivereminder.2"}, start)

					res, err = store.FetchNextReminders(context.Background(), req)
					require.NoError(t, err)
					require.NotNil(t, res)
					assertRemindersInResponse(t, res, []string{"type-B||type-B.221||type-B.221.1"}, start)

					res, err = store.FetchNextReminders(context.Background(), req)
					require.NoError(t, err)
					require.Empty(t, res)

					host := getHostForActor(t, store, "type-C", "type-C.inactivereminder")
					assert.Equal(t, "f4c7d514-3468-48dd-9103-297bf7fe91fd", host)
					host = getHostForActor(t, store, "type-B", "type-B.221")
					assert.Equal(t, "f4c7d514-3468-48dd-9103-297bf7fe91fd", host)
				})
			})
		})

		t.Run("Reminder leases", func(t *testing.T) {
			// Reload reminder test data
			t.Run("Load test data", func(t *testing.T) {
				td := GetTestData()
				require.NoError(t, store.LoadActorStateTestData(td), "Failed to load actor store test data")
				require.NoError(t, store.LoadReminderTestData(td), "Failed to load reminder test data")
			})
			require.False(t, t.Failed(), "Cannot continue if 'Load test data' test has failed")

			// Advance the time by 100ms
			require.NoError(t, store.AdvanceTime(100*time.Millisecond))

			// Note that "50d7623f-b165-4f9e-9f05-3b7a1280b222" is unhealthy
			hosts := []string{"7de434ce-e285-444f-9857-4d30cade3111", "50d7623f-b165-4f9e-9f05-3b7a1280b222", "f4c7d514-3468-48dd-9103-297bf7fe91fd"}
			actorTypes := []string{"type-A", "type-B"}

			var fetched []*actorstore.FetchedReminder
			t.Run("Fetch next 4 reminders", func(t *testing.T) {
				req := actorstore.FetchNextRemindersRequest{
					Hosts:      hosts,
					ActorTypes: actorTypes,
				}

				// type-A||type-A.11||type-A.11.1 is executed in 1s, so it is always included in the first set
				// The other ones are executed at the same time, so whether they are included in the first or second set isn't guaranteed
				res, err := store.FetchNextReminders(context.Background(), req)
				require.NoError(t, err)
				require.NotEmpty(t, res)
				fetched = append(fetched, res...)

				var ok bool
				for _, r := range res {
					if r.Key() == "type-A||type-A.11||type-A.11.1" {
						ok = true
					}
				}
				assert.True(t, ok, "Reminder type-A||type-A.11||type-A.11.1 must be found in the first response")

				res, err = store.FetchNextReminders(context.Background(), req)
				require.NoError(t, err)
				require.NotEmpty(t, res)
				fetched = append(fetched, res...)

				assertRemindersInResponse(t, fetched, []string{
					"type-A||type-A.11||type-A.11.2",
					"type-A||type-A.11||type-A.11.1",
					"type-A||type-A.inactivereminder||type-A.inactivereminder.1",
					"type-B||type-B.221||type-B.221.1",
				}, time.Time{})
			})

			// No point in continuing if the tests failed
			require.False(t, t.Failed(), "Cannot continue if previous test failed")

			t.Run("Get reminders with lease", func(t *testing.T) {
				for i := 0; i < len(fetched); i++ {
					res, err := store.GetReminderWithLease(context.Background(), fetched[i])
					require.NoErrorf(t, err, "Failed on fetched reminder %d", i)

					assert.Equalf(t, fetched[i].Key(), fmt.Sprintf("%s||%s||%s", res.ActorType, res.ActorID, res.Name), "Failed on fetched reminder %d", i)
					assert.InDeltaf(t, fetched[i].ScheduledTime().Unix(), res.ExecutionTime.Unix(), 1, "Failed on fetched reminder %d", i)
					assert.Nil(t, res.Period, "Failed on fetched reminder %d", i)
				}
			})

			// No point in continuing if the tests failed
			require.False(t, t.Failed(), "Cannot continue if previous test failed")

			t.Run("Update reminders with lease", func(t *testing.T) {
				// Advance the time by 1s
				require.NoError(t, store.AdvanceTime(time.Second))

				t.Run("Keeping lease for reminder 0", func(t *testing.T) {
					fr := fetched[0]
					// This also updates the lease expiration
					err := store.UpdateReminderWithLease(context.Background(), fr, actorstore.UpdateReminderWithLeaseRequest{
						ExecutionTime: fr.ScheduledTime(),
						Period:        ptr.Of("1m"),
						KeepLease:     true,
					})
					require.NoError(t, err)

					// Get the updated reminder
					res, err := store.GetReminderWithLease(context.Background(), fr)
					require.NoError(t, err)
					assert.Equal(t, fr.Key(), fmt.Sprintf("%s||%s||%s", res.ActorType, res.ActorID, res.Name))
					_ = assert.NotNil(t, res.Period) &&
						assert.Equal(t, "1m", *res.Period)
				})

				fr := fetched[1]
				updateReq := actorstore.UpdateReminderWithLeaseRequest{
					ExecutionTime: fr.ScheduledTime(),
					Period:        ptr.Of("1m"),
					KeepLease:     false,
				}

				t.Run("Relinquishing lease for reminder 1", func(t *testing.T) {
					// This causes the lease to be relinquished
					err := store.UpdateReminderWithLease(context.Background(), fr, updateReq)
					require.NoError(t, err)

					// Getting with lease should fail
					_, err = store.GetReminderWithLease(context.Background(), fr)
					require.Error(t, err)
					require.ErrorIs(t, err, actorstore.ErrReminderNotFound)

					// Get without a lease
					parts := strings.Split(fr.Key(), "||")
					require.Len(t, parts, 3)
					res, err := store.GetReminder(context.Background(), actorstore.ReminderRef{
						ActorType: parts[0],
						ActorID:   parts[1],
						Name:      parts[2],
					})
					require.NoError(t, err)
					_ = assert.NotNil(t, res.Period) &&
						assert.Equal(t, "1m", *res.Period)
				})

				// No point in continuing if the tests failed
				require.False(t, t.Failed(), "Cannot continue if previous test failed")

				t.Run("Errors when lease is invalid", func(t *testing.T) {
					// Reminder 1's lease was relinquished
					err := store.UpdateReminderWithLease(context.Background(), fr, updateReq)
					require.Error(t, err)
					require.ErrorIs(t, err, actorstore.ErrReminderNotFound)
				})
			})

			// No point in continuing if the tests failed
			require.False(t, t.Failed(), "Cannot continue if previous test failed")

			t.Run("Deleting reminders with lease", func(t *testing.T) {
				t.Run("Valid lease", func(t *testing.T) {
					fr := fetched[3]

					// Delete reminder 3 with a valid lease
					err := store.DeleteReminderWithLease(context.Background(), fr)
					require.NoError(t, err)

					// Reminder doesn't exist anymore
					_, err = store.GetReminderWithLease(context.Background(), fr)
					require.Error(t, err)
					require.ErrorIs(t, err, actorstore.ErrReminderNotFound)

					parts := strings.Split(fr.Key(), "||")
					require.Len(t, parts, 3)
					_, err = store.GetReminder(context.Background(), actorstore.ReminderRef{
						ActorType: parts[0],
						ActorID:   parts[1],
						Name:      parts[2],
					})
					require.Error(t, err)
					require.ErrorIs(t, err, actorstore.ErrReminderNotFound)
				})

				t.Run("Errors when lease is invalid", func(t *testing.T) {
					// Reminder 1's lease was relinquished
					err := store.DeleteReminderWithLease(context.Background(), fetched[1])
					require.Error(t, err)
					require.ErrorIs(t, err, actorstore.ErrReminderNotFound)
				})
			})

			// No point in continuing if the tests failed
			require.False(t, t.Failed(), "Cannot continue if previous test failed")

			t.Run("Leases expire", func(t *testing.T) {
				// Advance the time by RemindersLeaseDuration (we already advanced by 100ms earlier)
				require.NoError(t, store.AdvanceTime(actorsConfiguration.RemindersLeaseDuration))

				// Fetched reminders:
				// - 0 had its lease updated after 1s, so it should still be there
				// - The lease for 1 was already relinquished
				// - 2 should have its lease expired
				// - 3 was deleted
				for i := 0; i < 4; i++ {
					_, err := store.GetReminderWithLease(context.Background(), fetched[i])
					if i == 0 {
						require.NoErrorf(t, err, "Failed on fetched reinder: %d", i)
					} else {
						require.Errorf(t, err, "Failed on fetched reinder: %d", i)
						require.ErrorIsf(t, err, actorstore.ErrReminderNotFound, "Failed on fetched reinder: %d", i)
					}
				}
			})

			// No point in continuing if the tests failed
			require.False(t, t.Failed(), "Cannot continue if previous test failed")

			t.Run("Renew leases", func(t *testing.T) {
				count, err := store.RenewReminderLeases(context.Background(), actorstore.RenewReminderLeasesRequest{
					Hosts:      hosts,
					ActorTypes: actorTypes,
				})
				require.NoError(t, err)

				// We should have only 1 reinder with an active lease now
				require.Equal(t, int64(1), count)
				_, err = store.GetReminderWithLease(context.Background(), fetched[0])
				require.NoError(t, err)

				// Wait 2s (the reminder's lease remaining lifetime was just 1s)
				require.NoError(t, store.AdvanceTime(2*time.Second))

				// Reminder should still be there
				_, err = store.GetReminderWithLease(context.Background(), fetched[0])
				require.NoError(t, err)
			})

			t.Run("Relinquish lease", func(t *testing.T) {
				t.Run("Relinquish active lease", func(t *testing.T) {
					err := store.RelinquishReminderLease(context.Background(), fetched[0])
					require.NoError(t, err)

					_, err = store.GetReminderWithLease(context.Background(), fetched[0])
					require.Error(t, err)
					require.ErrorIs(t, err, actorstore.ErrReminderNotFound)
				})

				t.Run("Errors when lease is invalid", func(t *testing.T) {
					// The lease for reminder 1 is now NULL, so this will fail with a mismatching lease ID
					err := store.RelinquishReminderLease(context.Background(), fetched[1])
					require.Error(t, err)
					require.ErrorIs(t, err, actorstore.ErrReminderNotFound)
				})

				t.Run("Does not error when lease is expired", func(t *testing.T) {
					// Unlike the previous case, here the lease ID is valid, but the lease has expired
					// We don't error here because we sitll allow relinquishing the lease
					err := store.RelinquishReminderLease(context.Background(), fetched[2])
					require.NoError(t, err)
				})
			})

			t.Run("Create leased reminder", func(t *testing.T) {
				createReminderOpts := actorstore.ReminderOptions{
					ExecutionTime: store.GetTime().Add(time.Minute),
				}

				t.Run("Actor active on connected host", func(t *testing.T) {
					created, err := store.CreateLeasedReminder(context.Background(), actorstore.CreateLeasedReminderRequest{
						Reminder: actorstore.Reminder{
							ReminderRef: actorstore.ReminderRef{
								ActorType: "type-A",
								ActorID:   "type-A.11",
								Name:      "GiovanniGiorgio",
							},
							ReminderOptions: createReminderOpts,
						},
						Hosts: hosts,
					})
					require.NoError(t, err)
					require.NotNil(t, created)

					// Get the newly-stored reminder
					res, err := store.GetReminderWithLease(context.Background(), created)
					require.NoError(t, err)
					assert.Equal(t, created.Key(), fmt.Sprintf("%s||%s||%s", res.ActorType, res.ActorID, res.Name))
				})

				t.Run("Actor active on a non-connected host", func(t *testing.T) {
					// No error, but also no lease acquired
					created, err := store.CreateLeasedReminder(context.Background(), actorstore.CreateLeasedReminderRequest{
						Reminder: actorstore.Reminder{
							ReminderRef: actorstore.ReminderRef{
								ActorType: "type-B",
								ActorID:   "type-B.211",
								Name:      "GiovanniGiorgio",
							},
							ReminderOptions: createReminderOpts,
						},
						Hosts: hosts,
					})
					require.NoError(t, err)
					require.Nil(t, created)
				})

				t.Run("Actor is not active", func(t *testing.T) {
					// No error, but also no lease acquired
					created, err := store.CreateLeasedReminder(context.Background(), actorstore.CreateLeasedReminderRequest{
						Reminder: actorstore.Reminder{
							ReminderRef: actorstore.ReminderRef{
								ActorType: "type-A",
								ActorID:   "type-A.something-not-active",
								Name:      "GiovanniGiorgio",
							},
							ReminderOptions: createReminderOpts,
						},
						Hosts: hosts,
					})
					require.NoError(t, err)
					require.Nil(t, created)
				})
			})
		})
	}
}
