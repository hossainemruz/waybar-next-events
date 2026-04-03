package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/emruz-hossain/waybar-next-events/cmd"
	"github.com/emruz-hossain/waybar-next-events/pkg/types"
)

func main() {
	cmd.Execute()
	data := types.Result{
		Events: generateRandomEvents(),
	}
	if err := data.Print(); err != nil {
		fmt.Printf("{\"text\": \" Something went wrong!\", \"tooltip\": \"%s\"}\n", err)
	}
}

func generateRandomEvents() []types.Event {
	var events []types.Event

	titles := []string{
		"Team Standup",
		"Sprint Planning",
		"1:1 with Manager",
		"Design Review",
		"Lunch Break",
		"Code Review Session",
		"Product Sync",
		"Retrospective",
		"Architecture Discussion",
		"Customer Demo",
		"All Hands",
		"Focus Time",
		"Interview",
		"Onboarding Session",
		"Bug Triage",
	}

	calendars := []string{"Work", "Personal", "Team", "Project"}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	numDays := rand.Intn(7) + 1 // 1 to 7 days

	for day := range numDays {
		dayStart := today.AddDate(0, 0, day)
		numEvents := rand.Intn(4) + 1 // 1 to 4 events per day

		// For today, generate some past events
		if day == 0 {
			// Generate 1-3 past events earlier in the day
			numPast := rand.Intn(3) + 1
			// Start past events from 8 AM onwards
			pastHour := 8
			for range numPast {
				startHour := pastHour + rand.Intn(2)
				startMin := rand.Intn(4) * 15          // 0, 15, 30, or 45
				durationMin := (rand.Intn(4) + 1) * 30 // 30, 60, 90, or 120 minutes

				start := dayStart.Add(time.Duration(startHour)*time.Hour + time.Duration(startMin)*time.Minute)
				// Only add if the event ends before now
				end := start.Add(time.Duration(durationMin) * time.Minute)
				if end.After(now) {
					// Shift it further back so it ends before now
					start = now.Add(-time.Duration(durationMin+30) * time.Minute)
					end = start.Add(time.Duration(durationMin) * time.Minute)
				}

				events = append(events, types.Event{
					Title:    titles[rand.Intn(len(titles))],
					Start:    start,
					End:      end,
					Calendar: calendars[rand.Intn(len(calendars))],
				})
				pastHour = startHour + durationMin/60 + 1
			}
		}

		// For today, generate an ongoing event (50% chance)
		if day == 0 && rand.Intn(2) == 0 {
			// Start 10-45 minutes ago, end 15-60 minutes from now
			pastOffset := time.Duration(rand.Intn(36)+10) * time.Minute
			futureOffset := time.Duration(rand.Intn(46)+15) * time.Minute
			events = append(events, types.Event{
				Title:    titles[rand.Intn(len(titles))],
				Start:    now.Add(-pastOffset),
				End:      now.Add(futureOffset),
				Calendar: calendars[rand.Intn(len(calendars))],
			})
		}

		// Generate future events for the day
		var startHour int
		if day == 0 {
			// For today, start future events from the next hour
			startHour = now.Hour() + 1
			if startHour > 20 {
				continue // Too late in the day for more events
			}
		} else {
			startHour = 8 + rand.Intn(3) // 8-10 AM start
		}

		for range numEvents {
			if startHour > 20 {
				break
			}
			startMin := rand.Intn(4) * 15          // 0, 15, 30, or 45
			durationMin := (rand.Intn(4) + 1) * 30 // 30, 60, 90, or 120 minutes

			start := dayStart.Add(time.Duration(startHour)*time.Hour + time.Duration(startMin)*time.Minute)
			end := start.Add(time.Duration(durationMin) * time.Minute)

			events = append(events, types.Event{
				Title:    titles[rand.Intn(len(titles))],
				Start:    start,
				End:      end,
				Calendar: calendars[rand.Intn(len(calendars))],
			})
			startHour += durationMin/60 + 1
		}
	}

	return events
}
