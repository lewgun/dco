package main

import (
	"container/heap"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// number of client
	clientNums = 2

	// number of messages, simplify program implementation
	messageNums = 20

	// assume dataStreaming has unlimited capacity
	dataStreaming []chan data

	token int64
	step  int64

	maxSleepInterval int64 = 5
	maxGap           int64 = 10

	wg sync.WaitGroup
)

type data struct {
	kind    string
	prepare int64
	commit  int64
}

func init() {
	dataStreaming = make([]chan data, clientNums)
	for i := 0; i < clientNums; i++ {
		dataStreaming[i] = make(chan data, messageNums)
	}
}

func main() {
	wg.Add(clientNums*3 + 1)

	sigFinished := make([]chan struct{}, clientNums)

	// generateDatas and collect are parallel
	for i := 0; i < clientNums; i++ {
		sigFinished[i] = make(chan struct{}, 2)
		go func(index int) {
			defer wg.Done()
			generateDatas(index)

			sigFinished[index] <- struct{}{}

		}(i)
		go func(index int) {
			defer wg.Done()
			generateDatas(index)
			sigFinished[index] <- struct{}{}

		}(i)

		go func(index int) {
			defer wg.Done()
			<-sigFinished[index]
			<-sigFinished[index]
			close(dataStreaming[index])

		}(i)
	}

	go func() {
		defer wg.Done()
		collect()
	}()

	wg.Wait()

}

/*
 * generate prepare and commit datas.
 * assume max difference of send time between prepare and commit data is 2*maxSleepInterval(millisecond),
 * thus u would't think some extreme cases about thread starvation.
 */
func generateDatas(index int) {
	var maxToken int64
	for i := 0; i < messageNums; i++ {
		prepare := incrementToken()
		sleep(maxSleepInterval)

		dataStreaming[index] <- data{
			kind:    "prepare",
			prepare: prepare,
		}
		sleep(maxSleepInterval)

		commit := incrementToken()
		sleep(maxSleepInterval)

		dataStreaming[index] <- data{
			kind:    "commit",
			prepare: prepare,
			commit:  commit,
		}
		sleep(10 * maxSleepInterval)

		maxToken = commit
	}

	fmt.Println("max token", maxToken)
}

func incrementToken() int64 {
	return atomic.AddInt64(&token, rand.Int63()%maxGap+1)
}

func sleep(factor int64) {
	interval := atomic.AddInt64(&step, 3)%factor + 1
	waitTime := time.Duration(rand.Int63() % interval)
	time.Sleep(waitTime * time.Millisecond)
}

/*
 * 1. assume dataStreamings are endless => we have infinitely many datas;
 * because it's a simulation program, it has a limited number of datas, but the assumption is not shoul be satisfied
 * 2. sort commit kind of datas that are from multiple dataStreamings by commit ascending
 * and output them in the fastest way you can think
 */
func collect() {
	ch := merge(filter(dataStreaming))
	for v := range ch {
		fmt.Println(v)
	}
}

// filter split the endless raw data streaming into sections & sort & enqueue it
func filter(rawStreaming []chan data) []chan data {

	chRet := make([]chan data, len(rawStreaming))
	for i, ch := range rawStreaming {

		//just only one
		chRet[i] = make(chan data, 1)

		go func(index int, ch <-chan data) {

			var (
				buf        []data
				prevData   data
				minPrepare int64
			)

			for curData := range ch {
				if curData.kind == "commit" {
					buf = append(buf, curData)
				}

				// 1. split the endless data stream into sections by prepare token
				// 2. sort the sections
				// 3. enqueue the sorted sections
				// see README.md for details
				if prevData.kind == "prepare" && curData.kind == "prepare" {
					minPrepare = prevData.prepare
					if minPrepare > curData.prepare {
						minPrepare = curData.prepare
					}
					if len(buf) != 0 {
						insertionSort(buf)
						for _, val := range buf {
							chRet[index] <- val
						}
						buf = nil
					}

				}

				prevData = curData
			}

			//the last section
			if len(buf) != 0 {
				insertionSort(buf)
				for _, val := range buf {
					chRet[index] <- val
				}
				buf = nil
			}
			close(chRet[index])

		}(i, ch)

	}

	return chRet
}

func buildPriorityQueue(nodes []data) priorityQueue {
	pq := make(priorityQueue, len(nodes))

	for i, v := range nodes {
		pq[i] = &pqItem{
			data:  v,
			index: i,
		}
	}

	heap.Init(&pq)
	return pq
}

// merge the all individually data streams into the final stream
func merge(streaming []chan data) <-chan data {

	chRet := make(chan data, clientNums)

	go func(ch chan data) {
		// commit token => client number
		m := map[int64]int{}

		size := len(streaming)

		//the priority queue's initialize datas
		nodes := make([]data, size)

		var wg sync.WaitGroup

		// pick the all clients' first commit data
		for i := 0; i < size; i++ {
			wg.Add(1)

			go func(j int) {
				val := <-streaming[j]

				// which client is the commit token come from
				m[val.commit] = j

				nodes[j] = val
				wg.Done()
			}(i)
		}

		wg.Wait()

		pq := buildPriorityQueue(nodes)

		var idx int
		for pq.Len() > 0 {
			val := heap.Pop(&pq).(*pqItem)

			chRet <- val.data

			idx = m[val.commit]

			delete(m, val.commit)

			if streaming[idx] == nil {
				continue
			}

			//get next value from the outputed data's owner
			v, ok := <-streaming[idx]
			if !ok {

				// NOTE:
				// if the stream is closed, set it to nil, not remove it
				streaming[idx] = nil
				continue
			}

			heap.Push(&pq, &pqItem{
				data: v,
			})

			//update the mapping
			m[v.commit] = idx

		}

		close(chRet)

	}(chRet)

	return chRet

}
