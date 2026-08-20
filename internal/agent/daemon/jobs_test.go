package daemon

import (
	"context"
	"testing"
)

func TestProjectJobsAreIdempotentAndExclusive(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	daemon := &Daemon{sessionCtx: parent, jobs: make(map[string]*activeJob)}

	first, start, err := daemon.beginJob("api", "release-1")
	if err != nil || !start {
		t.Fatalf("first job: start=%v err=%v", start, err)
	}
	same, start, err := daemon.beginJob("api", "release-1")
	if err != nil || start || same != first {
		t.Fatalf("duplicate job: same=%v start=%v err=%v", same == first, start, err)
	}
	if _, _, err := daemon.beginJob("api", "release-2"); err == nil {
		t.Fatal("a second job for the same project was accepted")
	}

	cancel()
	select {
	case <-first.ctx.Done():
	default:
		t.Fatal("job survived its agent session")
	}

	daemon.endJob("api", first)
	if _, exists := daemon.jobs["api"]; exists {
		t.Fatal("completed job remained active")
	}
}
