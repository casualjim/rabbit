package tasks

import (
	"context"
	"log"
	"sync"
	"testing"
	"time"
)

type ctxExamplesKey string

func TestSimpleContext(t *testing.T) {
	wg := &sync.WaitGroup{}

	start := time.Now()
	estop := make(chan time.Time, 1)
	ectx := context.Background()

	ctx, cancel := context.WithCancel(ectx) // with cancel needed to be able to do
	stop1 := make(chan time.Time, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				log.Println("done ctx")
				stop1 <- time.Now()
				return
			case <-time.After(1 * time.Second):
				log.Println("iteration for ctx")
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer func() {
			estop <- time.Now()
			wg.Done()
		}()
		for {
			select {
			case <-ectx.Done():
				log.Println("done ectx, should not occur")
				return
			case <-time.After(1 * time.Second):
				log.Println("iteration for ectx because nil channel blocks forever")
				select {
				case <-ctx.Done():
					log.Println("done ectx through escaping with ctx.Done()")
					return
				default:
				}
			}
		}
	}()

	ctx2 := context.WithValue(ctx, ctxExamplesKey("ctx2val"), 0)
	stop2 := make(chan time.Time, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx2.Done():
				log.Println("done ctx2")
				stop2 <- time.Now()
				return
			default:
				time.Sleep(2 * time.Second)
				log.Println("iteration for ctx2")
				ctx2 = context.WithValue(ctx2, ctxExamplesKey("ctx2val"), ctx2.Value(ctxExamplesKey("ctx2val")).(int)+1)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-time.After(9 * time.Second)
		cancel()
		log.Println("iteration for ctx")
	}()

	wg.Wait()

	log.Printf("ctx2val (ctx): %v", ctx.Value(ctxExamplesKey("ctx2val")))
	log.Printf("ctx2val (ctx2): %d", ctx2.Value(ctxExamplesKey("ctx2val")))
	log.Println("took", time.Now().Sub(start))
	log.Printf("ectx took: %v", (<-estop).Sub(start))
	log.Printf("ctx took: %v", (<-stop1).Sub(start))
	log.Printf("ctx2 took: %v", (<-stop2).Sub(start))
}
