package wailsui

import "github.com/AlexGalhardo/kali-tools/internal/job"

func EventPayloadForTest(event job.Event) (string, any, bool) { return eventPayload(event) }
