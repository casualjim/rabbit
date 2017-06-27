///////////////////////////////////////////////////////////////////////
// Copyright (C) 2016 VMware, Inc. All rights reserved.
// -- VMware Confidential
///////////////////////////////////////////////////////////////////////
package workerGroup

import (
	"sync"

	"golang.org/x/sync/syncmap"
)

type WorkerGroup interface {
	MakeSink() chan interface{}
	Add(int)
	Done()
	Wait()
	Fetch(chan interface{}) []interface{}
}

func NewWorkerGroup() WorkerGroup {
	return &defautlWorkerGroup{
		wg:         new(sync.WaitGroup),
		wgInternal: new(sync.WaitGroup),
		info:       new(syncmap.Map),
	}
}

type defautlWorkerGroup struct {
	wg         *sync.WaitGroup
	wgInternal *sync.WaitGroup
	info       *syncmap.Map
}

func (d *defautlWorkerGroup) MakeSink() (inputChan chan interface{}) {
	inputChan = make(chan interface{})
	d.wgInternal.Add(1)
	go func() {
		var results []interface{}
		d.info.Store(inputChan, results)
		for i := range inputChan {
			results = append(results, i)
		}
		d.info.Store(inputChan, results)
		d.wgInternal.Done()
	}()
	return
}

func (d *defautlWorkerGroup) Add(delta int) {
	d.wg.Add(delta)
}

func (d *defautlWorkerGroup) Done() {
	d.wg.Done()
}

func (d *defautlWorkerGroup) Wait() {
	d.wg.Wait()
	d.info.Range(func(key, value interface{}) bool {
		inputChan, ok := key.(chan interface{})
		if ok {
			close(inputChan)
		}
		return true

	})
	d.wgInternal.Wait()
}

// Fetch must be called after Wait
func (d *defautlWorkerGroup) Fetch(key chan interface{}) []interface{} {
	v, _ := d.info.Load(key)
	r, ok := v.([]interface{})
	if ok {
		return r
	}
	return nil
}
