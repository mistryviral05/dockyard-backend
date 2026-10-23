package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

func main() {
	cores := flag.Int("cores", runtime.NumCPU(), "how many cores to keep busy")
	seconds := flag.Int("seconds", 60, "how long to run")
	flag.Parse()

	if *cores < 1 {
		*cores = 1
	}
	if *cores > runtime.NumCPU() {
		*cores = runtime.NumCPU()
	}

	runtime.GOMAXPROCS(*cores)

	stop := make(chan struct{})
	var once sync.Once
	closeStop := func() { once.Do(func() { close(stop) }) }

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	fmt.Printf("burning %d of %d cores for %ds, press ctrl+c to stop early\n",
		*cores, runtime.NumCPU(), *seconds)

	var wg sync.WaitGroup
	for i := 0; i < *cores; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			burn(stop)
		}()
	}

	timer := time.NewTimer(time.Duration(*seconds) * time.Second)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-signals:
		fmt.Println("stopping early")
	}

	closeStop()
	wg.Wait()

	fmt.Println("done")
}

func burn(stop <-chan struct{}) {
	x := 1.0
	for {
		select {
		case <-stop:
			return
		default:
			for i := 0; i < 1_000_000; i++ {
				x = math.Sqrt(x*1.0000001 + float64(i))
			}
		}
	}
}
