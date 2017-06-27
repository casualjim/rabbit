///////////////////////////////////////////////////////////////////////
// Copyright (C) 2016 VMware, Inc. All rights reserved.
// -- VMware Confidential
///////////////////////////////////////////////////////////////////////
package workerGroup

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func sleepRandomMillisecond(upperLimit int) {
	time.Sleep(time.Duration(rand.Intn(upperLimit)) * time.Millisecond)
}

func TestWorkerGroup(t *testing.T) {
	assert := assert.New(t)

	workerGroup := NewWorkerGroup()

	data1 := workerGroup.MakeSink() // chan int
	data2 := workerGroup.MakeSink() // chan string
	data3 := workerGroup.MakeSink() // chan struct {Name string}
	testCasesNum := 10
	for i := 0; i < testCasesNum; i++ {
		workerGroup.Add(1)
		go func(i int) {
			data1 <- i
			data2 <- fmt.Sprintf("%d", i)
			data3 <- struct{ Name string }{Name: fmt.Sprintf("from %d", i)}
			workerGroup.Done()
		}(i)
	}
	workerGroup.Wait()
	data1Result := workerGroup.Fetch(data1)
	data2Result := workerGroup.Fetch(data2)
	data3Result := workerGroup.Fetch(data3)
	assert.Len(data1Result, testCasesNum)
	assert.Len(data2Result, testCasesNum)
	assert.Len(data3Result, testCasesNum)

}

func TestWorkerGroupRunningLong(t *testing.T) {
	assert := assert.New(t)

	workerGroup := NewWorkerGroup()

	data1 := workerGroup.MakeSink() // chan int
	data2 := workerGroup.MakeSink() // chan string
	data3 := workerGroup.MakeSink() // chan struct {Name string}
	data4 := workerGroup.MakeSink() // chan struct {Name string}
	data5 := workerGroup.MakeSink() // chan struct {Name string}
	testCasesNum := 50
	for i := 0; i < testCasesNum; i++ {
		workerGroup.Add(1)
		go func(i int) {
			sleepRandomMillisecond(100)
			data1 <- i
			sleepRandomMillisecond(100)
			data2 <- fmt.Sprintf("%d", i)
			sleepRandomMillisecond(100)
			data3 <- struct{ Name string }{Name: fmt.Sprintf("from %d", i)}
			if i%2 == 0 {
				sleepRandomMillisecond(100)
				data4 <- fmt.Sprintf("from data4 %d", i)
			}
			if i > 10 && i <= 25 {
				sleepRandomMillisecond(100)
				data5 <- i
			}
			workerGroup.Done()
		}(i)
	}
	workerGroup.Wait()
	data1Result := workerGroup.Fetch(data1)
	data2Result := workerGroup.Fetch(data2)
	data3Result := workerGroup.Fetch(data3)
	data4Result := workerGroup.Fetch(data4)
	data5Result := workerGroup.Fetch(data5)
	assert.Len(data1Result, testCasesNum)
	assert.Len(data2Result, testCasesNum)
	assert.Len(data3Result, testCasesNum)
	assert.Len(data4Result, testCasesNum/2)
	assert.Len(data5Result, 15)
	var data5ResultTmp []int
	for _, i := range data5Result {
		r, ok := i.(int)
		if ok {
			data5ResultTmp = append(data5ResultTmp, r)
		}
	}
	sort.Ints(data5ResultTmp)
	assert.Len(data5ResultTmp, 15)
	for i, r := range data5ResultTmp {
		assert.Equal(i+11, r)
	}
}
