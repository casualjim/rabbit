// +build !race

package future_test

import (
	"sync"
	"testing"

	"github.com/casualjim/rabbit/future"
)

func TestCancelConc(t *testing.T) {
	loop := func() {
		const N = 8000
		start := make(chan int)
		var done sync.WaitGroup
		done.Add(N)
		f := future.Do(future.Func(func() (future.Value, error) {
			select {} //block
			return 1, nil
		}))
		for i := 0; i < N; i++ {
			go func() {
				defer done.Done()
				<-start
				f.Cancel()
			}()
		}
		close(start)
		done.Wait()
	}

	for i := 0; i < 500; i++ {
		loop()
	}

}
