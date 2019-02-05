package flip

import (
	"context"
	"fmt"
	clockwork "github.com/keybase/clockwork"
	"testing"
)

type testDealersHelper struct {
	clock clockwork.FakeClock
}

func newTestDealersHelper() *testDealersHelper {
	return &testDealersHelper{clock: clockwork.NewFakeClock()}
}

func (t *testDealersHelper) Clock() clockwork.Clock {
	return t.clock
}

func (t *testDealersHelper) CLogf(ctx context.Context, fmtString string, args ...interface{}) {
	fmt.Printf(fmtString, args...)
}

func TestDealer(t *testing.T) {
	dh := newTestDealersHelper()
	dealer := NewDealer(dh)
	ctx := context.Background()
	go func() {
		dealer.Run(ctx)
	}()
	<-dealer.UpdateCh()
}
